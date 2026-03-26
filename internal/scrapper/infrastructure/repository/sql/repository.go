package sqlrepo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
)

const (
	linkCheckInterval = "5 minutes"
)

type SqlRepository struct {
	db    *pgxpool.Pool
	sqlDB *sql.DB
}

func NewRepository(connString string) (*SqlRepository, error) {
	db, err := pgxpool.New(context.Background(), connString)
	if err != nil {
		return nil, fmt.Errorf("connect to db: %w", err)
	}

	sqlDB, err := sql.Open("pgx", connString)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("open sql db: %w", err)
	}

	return &SqlRepository{db: db, sqlDB: sqlDB}, nil
}

func (r *SqlRepository) Close() {
	if r.db != nil {
		r.db.Close()
	}
	if r.sqlDB != nil {
		_ = r.sqlDB.Close()
	}
}

func (r *SqlRepository) DB() *sql.DB {
	return r.sqlDB
}

func (r *SqlRepository) RegisterChat(ctx context.Context, chatID int64) error {
	_, err := r.db.Exec(ctx, `
        INSERT INTO chats (id) VALUES ($1) ON CONFLICT (id) DO NOTHING
    `, chatID)
	if err != nil {
		return fmt.Errorf("register chat: %w", err)
	}
	return nil
}

func (r *SqlRepository) DeleteChat(ctx context.Context, chatID int64) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()
	rows, err := tx.Query(ctx, `
        SELECT link_id FROM link_chat WHERE chat_id = $1
    `, chatID)
	if err != nil {
		return fmt.Errorf("get subscriptions: %w", err)
	}

	var linkIDs []int64
	for rows.Next() {
		var linkID int64
		if err = rows.Scan(&linkID); err != nil {
			rows.Close()
			return fmt.Errorf("scan link_id: %w", err)
		}
		linkIDs = append(linkIDs, linkID)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("rows error: %w", err)
	}
	rows.Close()

	// удаление чата
	_, err = tx.Exec(ctx, `DELETE FROM chats WHERE id = $1`, chatID)
	if err != nil {
		return fmt.Errorf("delete chat: %w", err)
	}

	// удаление ссылок, у которых больше нет подписчиков
	for _, linkID := range linkIDs {
		var count int
		err = tx.QueryRow(ctx, `
            SELECT COUNT(*) FROM link_chat WHERE link_id = $1
        `, linkID).Scan(&count)
		if err != nil {
			return fmt.Errorf("check subscribers count: %w", err)
		}
		if count == 0 {
			_, err = tx.Exec(ctx, `DELETE FROM links WHERE id = $1`, linkID)
			if err != nil {
				return fmt.Errorf("delete link: %w", err)
			}
		}
	}

	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}

	return nil
}

func (r *SqlRepository) ChatExists(ctx context.Context, chatID int64) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM chats WHERE id = $1)`, chatID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check chat exists: %w", err)
	}
	return exists, nil
}

func (r *SqlRepository) AddLink(ctx context.Context, chatID int64, url string, tags []string) (domain.Link, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return domain.Link{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	// добавление ссылки или получение существующей
	var linkID int64
	err = tx.QueryRow(ctx, `
        INSERT INTO links (url, updated_at) VALUES ($1, $2) 
        ON CONFLICT (url) DO UPDATE SET url = EXCLUDED.url
        RETURNING id
    `, url, time.Now()).Scan(&linkID)
	if err != nil {
		return domain.Link{}, fmt.Errorf("upsert link: %w", err)
	}

	// проверка отслеживает ли уже пользователь ссылку
	var alreadySubscribed bool
	err = tx.QueryRow(ctx, `
        SELECT EXISTS(SELECT 1 FROM link_chat WHERE chat_id = $1 AND link_id = $2)
    `, chatID, linkID).Scan(&alreadySubscribed)
	if err != nil {
		return domain.Link{}, fmt.Errorf("check subscription: %w", err)
	}
	if alreadySubscribed {
		return domain.Link{}, errors.New("link already tracked")
	}

	// подписывание пользователя на ссылку
	_, err = tx.Exec(ctx, `
        INSERT INTO link_chat (chat_id, link_id) VALUES ($1, $2) ON CONFLICT DO NOTHING
    `, chatID, linkID)
	if err != nil {
		return domain.Link{}, fmt.Errorf("create subscription: %w", err)
	}

	// теги
	for _, tag := range tags {
		var tagID int64
		err = tx.QueryRow(ctx, `
            INSERT INTO tags (name) VALUES ($1) ON CONFLICT (name) DO UPDATE SET name = EXCLUDED.name RETURNING id
        `, tag).Scan(&tagID)
		if err != nil {
			return domain.Link{}, fmt.Errorf("upsert tag: %w", err)
		}

		_, err = tx.Exec(ctx, `
            INSERT INTO link_tags (chat_id, link_id, tag_id) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING
        `, chatID, linkID, tagID)
		if err != nil {
			return domain.Link{}, fmt.Errorf("link tag: %w", err)
		}
	}

	// получаем итоговую ссылку
	var link domain.Link
	err = tx.QueryRow(ctx, `
        SELECT id, url, updated_at FROM links WHERE id = $1
    `, linkID).Scan(&link.ID, &link.URL, &link.UpdatedAt)
	if err != nil {
		return domain.Link{}, fmt.Errorf("get link: %w", err)
	}

	if err = tx.Commit(ctx); err != nil {
		return domain.Link{}, fmt.Errorf("commit tx: %w", err)
	}

	link.Tags = tags
	return link, nil
}

func (r *SqlRepository) RemoveLink(ctx context.Context, chatID int64, url string) (domain.Link, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return domain.Link{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	var linkID int64
	err = tx.QueryRow(ctx, `SELECT id FROM links WHERE url = $1`, url).Scan(&linkID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Link{}, errors.New("link not found")
		}
		return domain.Link{}, fmt.Errorf("get link: %w", err)
	}

	var link domain.Link
	err = tx.QueryRow(ctx, `SELECT id, url, updated_at FROM links WHERE id = $1`, linkID).
		Scan(&link.ID, &link.URL, &link.UpdatedAt)
	if err != nil {
		return domain.Link{}, fmt.Errorf("get link info: %w", err)
	}

	cmdTag, err := tx.Exec(ctx, `DELETE FROM link_chat WHERE chat_id = $1 AND link_id = $2`, chatID, linkID)
	if err != nil {
		return domain.Link{}, fmt.Errorf("delete subscription: %w", err)
	}
	if cmdTag.RowsAffected() == 0 {
		return domain.Link{}, errors.New("subscription not found")
	}

	var count int
	err = tx.QueryRow(ctx, `SELECT COUNT(*) FROM link_chat WHERE link_id = $1`, linkID).Scan(&count)
	if err != nil {
		return domain.Link{}, fmt.Errorf("count subscribers: %w", err)
	}

	if count == 0 {
		_, err = tx.Exec(ctx, `DELETE FROM links WHERE id = $1`, linkID)
		if err != nil {
			return domain.Link{}, fmt.Errorf("delete link: %w", err)
		}
	}

	_, err = tx.Exec(ctx, `
        DELETE FROM tags
        WHERE id NOT IN (SELECT DISTINCT tag_id FROM link_tags)
    `)
	if err != nil {
		return domain.Link{}, fmt.Errorf("clean unused tags: %w", err)
	}

	if err = tx.Commit(ctx); err != nil {
		return domain.Link{}, fmt.Errorf("commit tx: %w", err)
	}

	return link, nil
}

func (r *SqlRepository) GetLinks(ctx context.Context, chatID int64, limit, offset int) ([]domain.Link, error) {
	rows, err := r.db.Query(ctx, `
        SELECT l.id, l.url, l.updated_at,
            ARRAY_AGG(DISTINCT t.name) FILTER (WHERE t.name IS NOT NULL) as tags
        FROM links l
        JOIN link_chat lc ON l.id = lc.link_id
        LEFT JOIN link_tags lt ON lc.chat_id = lt.chat_id AND lc.link_id = lt.link_id
        LEFT JOIN tags t ON lt.tag_id = t.id
        WHERE lc.chat_id = $1
        GROUP BY l.id, l.url, l.updated_at
        ORDER BY l.id
		LIMIT $2 OFFSET $3
    `, chatID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("query links: %w", err)
	}
	defer rows.Close()

	var links []domain.Link
	for rows.Next() {
		var link domain.Link
		var tags []string
		err = rows.Scan(&link.ID, &link.URL, &link.UpdatedAt, &tags)
		if err != nil {
			return nil, fmt.Errorf("scan link: %w", err)
		}
		link.Tags = tags
		links = append(links, link)
	}
	return links, nil
}

func (r *SqlRepository) GetAllLinks(ctx context.Context, limit, offset int) ([]domain.Link, error) {
	rows, err := r.db.Query(ctx, `
        SELECT id, url, updated_at FROM links 
		WHERE last_checked_at < NOW() - $1::interval
		ORDER BY id LIMIT $2 OFFSET $3
    `, linkCheckInterval, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("query all links: %w", err)
	}
	defer rows.Close()

	var links []domain.Link
	for rows.Next() {
		var link domain.Link
		err = rows.Scan(&link.ID, &link.URL, &link.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("scan link: %w", err)
		}
		links = append(links, link)
	}
	return links, nil
}

func (r *SqlRepository) GetSubscribers(ctx context.Context, url string) ([]int64, error) {
	rows, err := r.db.Query(ctx, `
        SELECT lc.chat_id
        FROM links l
        JOIN link_chat lc ON l.id = lc.link_id
        WHERE l.url = $1
    `, url)
	if err != nil {
		return nil, fmt.Errorf("query subscribers: %w", err)
	}
	defer rows.Close()

	var chatIDs []int64
	for rows.Next() {
		var id int64
		if err = rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan chat_id: %w", err)
		}
		chatIDs = append(chatIDs, id)
	}
	return chatIDs, nil
}

func (r *SqlRepository) UpdateTimestamp(ctx context.Context, url string, timestamp time.Time) error {
	_, err := r.db.Exec(ctx, `
        UPDATE links SET updated_at = $1 WHERE url = $2
    `, timestamp, url)
	if err != nil {
		return fmt.Errorf("update timestamp: %w", err)
	}
	return nil
}

func (r *SqlRepository) UpdateLastChecked(ctx context.Context, url string, timestamp time.Time) error {
	_, err := r.db.Exec(ctx, `
        UPDATE links SET last_checked_at = $1 WHERE url = $2
    `, timestamp, url)
	if err != nil {
		return fmt.Errorf("update last checked: %w", err)
	}
	return err
}
