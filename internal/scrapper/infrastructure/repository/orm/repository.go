package ormrepo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/doug-martin/goqu/v9"
	"github.com/jackc/pgx/pgtype"
	_ "github.com/jackc/pgx/v5/stdlib" // Регистрирует драйвер pgx для database/sql
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
)

type OrmRepository struct {
	sqlDB *sql.DB
	db    *goqu.Database
}

func NewRepository(connString string) (*OrmRepository, error) {
	sqlDB, err := sql.Open("pgx", connString)
	if err != nil {
		return nil, fmt.Errorf("connect to db: %w", err)
	}

	return &OrmRepository{
		sqlDB: sqlDB,
		db:    goqu.New("postgres", sqlDB),
	}, nil
}

func (r *OrmRepository) Close() {
	_ = r.sqlDB.Close()
}

func (r *OrmRepository) DB() *sql.DB {
	return r.sqlDB
}

func (r *OrmRepository) RegisterChat(ctx context.Context, chatID int64) error {
	_, err := r.db.Insert("chats").
		Rows(goqu.Record{"id": chatID}).
		OnConflict(goqu.DoNothing()).
		Executor().ExecContext(ctx)
	if err != nil {
		return fmt.Errorf("register chat: %w", err)
	}

	return nil
}

func (r *OrmRepository) DeleteChat(ctx context.Context, chatID int64) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	var linkIDs []int64
	err = tx.From("link_chat").
		Select("link_id").
		Where(goqu.Ex{"chat_id": chatID}).
		ScanValsContext(ctx, &linkIDs)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("get subscriptions: %w", err)
	}

	_, err = tx.Delete("chats").
		Where(goqu.Ex{"id": chatID}).
		Executor().
		ExecContext(ctx)
	if err != nil {
		return fmt.Errorf("delete chat: %w", err)
	}

	for _, linkID := range linkIDs {
		var count int
		_, err = tx.From("link_chat").
			Select(goqu.COUNT("*")).
			Where(goqu.Ex{"link_id": linkID}).
			ScanValContext(ctx, &count)
		if err != nil {
			return fmt.Errorf("check subscribers count for link %d: %w", linkID, err)
		}

		if count == 0 {
			_, err = tx.Delete("links").
				Where(goqu.Ex{"id": linkID}).
				Executor().
				ExecContext(ctx)
			if err != nil {
				return fmt.Errorf("delete link %d: %w", linkID, err)
			}
		}
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

func (r *OrmRepository) ChatExists(ctx context.Context, chatID int64) (bool, error) {
	var exists bool

	_, err := r.db.From("chats").
		Select(goqu.L("EXISTS (SELECT 1 FROM chats WHERE id = ?)", chatID)).
		ScanValContext(ctx, &exists)
	if err != nil {
		return false, fmt.Errorf("check chat exists: %w", err)
	}

	return exists, nil
}

func (r *OrmRepository) AddLink(ctx context.Context, chatID int64, url string, tags []string) (domain.Link, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return domain.Link{}, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	var linkID int64

	_, err = tx.Insert("links").
		Rows(goqu.Record{
			"url":        url,
			"updated_at": time.Now(),
		}).
		OnConflict(goqu.DoUpdate("url", goqu.Record{"url": goqu.I("links.url")})).
		Returning("id").
		Executor().ScanValContext(ctx, &linkID)
	if err != nil {
		return domain.Link{}, fmt.Errorf("upsert link: %w", err)
	}

	var exists bool
	_, err = tx.From("link_chat").
		Select(goqu.L("EXISTS (SELECT 1 FROM link_chat WHERE chat_id=? AND link_id=?)", chatID, linkID)).
		ScanValContext(ctx, &exists)
	if err != nil {
		return domain.Link{}, fmt.Errorf("check subscription: %w", err)
	}
	if exists {
		return domain.Link{}, errors.New("link already tracked")
	}

	_, err = tx.Insert("link_chat").
		Rows(goqu.Record{"chat_id": chatID, "link_id": linkID}).
		OnConflict(goqu.DoNothing()).
		Executor().ExecContext(ctx)
	if err != nil {
		return domain.Link{}, fmt.Errorf("create subscription: %w", err)
	}

	for _, tag := range tags {
		var tagID int64

		_, err = tx.Insert("tags").
			Rows(goqu.Record{"name": tag}).
			OnConflict(goqu.DoUpdate("name", goqu.Record{"name": goqu.I("tags.name")})).
			Returning("id").Executor().
			ScanValContext(ctx, &tagID)
		if err != nil {
			return domain.Link{}, fmt.Errorf("upsert tag %q: %w", tag, err)
		}

		_, err = tx.Insert("link_tags").
			Rows(goqu.Record{
				"chat_id": chatID,
				"link_id": linkID,
				"tag_id":  tagID,
			}).
			OnConflict(goqu.DoNothing()).
			Executor().ExecContext(ctx)
		if err != nil {
			return domain.Link{}, fmt.Errorf("attach tag %q: %w", tag, err)
		}
	}

	var link domain.Link
	_, err = tx.From("links").
		Select("id", "url", "updated_at").
		Where(goqu.Ex{"id": linkID}).
		ScanStructContext(ctx, &link)
	if err != nil {
		return domain.Link{}, fmt.Errorf("get link: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return domain.Link{}, fmt.Errorf("commit transaction: %w", err)
	}

	link.Tags = tags
	return link, nil
}

func (r *OrmRepository) RemoveLink(ctx context.Context, chatID int64, url string) (domain.Link, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return domain.Link{}, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	var link domain.Link

	found, err := tx.From("links").
		Select("id", "url", "updated_at").
		Where(goqu.Ex{"url": url}).
		ScanStructContext(ctx, &link)
	if err != nil {
		return domain.Link{}, fmt.Errorf("get link by url: %w", err)
	}
	if !found {
		return domain.Link{}, errors.New("link not found")
	}

	res, err := tx.Delete("link_chat").
		Where(goqu.Ex{
			"chat_id": chatID,
			"link_id": link.ID,
		}).
		Executor().ExecContext(ctx)
	if err != nil {
		return domain.Link{}, fmt.Errorf("delete subscription: %w", err)
	}

	count, err := res.RowsAffected()
	if err != nil {
		return domain.Link{}, fmt.Errorf("get rows affected: %w", err)
	}
	if count == 0 {
		return domain.Link{}, errors.New("subscription not found")
	}

	var subscribersCount int
	_, err = tx.From("link_chat").
		Select(goqu.COUNT("*")).
		Where(goqu.Ex{"link_id": link.ID}).
		ScanValContext(ctx, &subscribersCount)
	if err != nil {
		return domain.Link{}, fmt.Errorf("count subscribers: %w", err)
	}

	if subscribersCount == 0 {
		_, err = tx.Delete("links").
			Where(goqu.Ex{"id": link.ID}).
			Executor().ExecContext(ctx)
		if err != nil {
			return domain.Link{}, fmt.Errorf("delete link: %w", err)
		}
	}

	_, err = tx.Delete("tags").
		Where(goqu.L("id NOT IN (SELECT DISTINCT tag_id FROM link_tags)")).
		Executor().ExecContext(ctx)
	if err != nil {
		return domain.Link{}, fmt.Errorf("clean unused tags: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return domain.Link{}, fmt.Errorf("commit transaction: %w", err)
	}
	return link, nil
}

func (r *OrmRepository) GetLinks(ctx context.Context, chatID int64, limit, offset int) ([]domain.Link, error) {
	type row struct {
		ID        int64            `db:"id"`
		URL       string           `db:"url"`
		UpdatedAt time.Time        `db:"updated_at"`
		Tags      pgtype.TextArray `db:"tags"`
	}

	var rows []row

	err := r.db.
		From(goqu.T("links").As("l")).
		Join(
			goqu.T("link_chat").As("lc"),
			goqu.On(goqu.I("l.id").Eq(goqu.I("lc.link_id"))),
		).
		LeftJoin(
			goqu.T("link_tags").As("lt"),
			goqu.On(
				goqu.I("lc.chat_id").Eq(goqu.I("lt.chat_id")),
				goqu.I("lc.link_id").Eq(goqu.I("lt.link_id")),
			),
		).
		LeftJoin(
			goqu.T("tags").As("t"),
			goqu.On(goqu.I("lt.tag_id").Eq(goqu.I("t.id"))),
		).
		Select(
			goqu.I("l.id"),
			goqu.I("l.url"),
			goqu.I("l.updated_at"),
			goqu.L("ARRAY_AGG(DISTINCT t.name) FILTER (WHERE t.name IS NOT NULL)").As("tags"),
		).
		Where(goqu.I("lc.chat_id").Eq(chatID)).
		GroupBy("l.id", "l.url", "l.updated_at").
		Order(goqu.I("l.id").Asc()).
		Limit(uint(limit)).
		Offset(uint(offset)).
		ScanStructsContext(ctx, &rows)
	if err != nil {
		return nil, fmt.Errorf("query links: %w", err)
	}

	var result []domain.Link
	for _, r := range rows {
		tags := make([]string, len(r.Tags.Elements))
		for i, el := range r.Tags.Elements {
			tags[i] = el.String
		}

		result = append(result, domain.Link{
			ID:        r.ID,
			URL:       r.URL,
			UpdatedAt: r.UpdatedAt,
			Tags:      tags,
		})
	}

	return result, nil
}

func (r *OrmRepository) GetLinksWithInterval(ctx context.Context, limit, offset int, interval time.Duration) ([]domain.Link, error) {
	var links []domain.Link

	err := r.db.
		From("links").
		Select("id", "url", "updated_at").
		Where(goqu.L("last_checked_at < now() - interval ?", fmt.Sprintf("%.0f seconds", interval.Seconds()))).
		Order(goqu.I("id").Asc()).
		Limit(uint(limit)).
		Offset(uint(offset)).
		ScanStructsContext(ctx, &links)
	if err != nil {
		return nil, fmt.Errorf("query all links: %w", err)
	}

	return links, nil
}

func (r *OrmRepository) GetSubscribers(ctx context.Context, url string) ([]int64, error) {
	var ids []int64

	err := r.db.
		From(goqu.T("links").As("l")).
		Join(
			goqu.T("link_chat").As("lc"),
			goqu.On(goqu.I("l.id").Eq(goqu.I("lc.link_id"))),
		).
		Select(goqu.I("lc.chat_id")).
		Where(goqu.I("l.url").Eq(url)).
		ScanValsContext(ctx, &ids)
	if err != nil {
		return nil, fmt.Errorf("query subscribers: %w", err)
	}

	return ids, nil
}

func (r *OrmRepository) UpdateTimestamp(ctx context.Context, url string, ts time.Time) error {
	_, err := r.db.
		Update("links").
		Set(goqu.Record{"updated_at": ts}).
		Where(goqu.Ex{"url": url}).
		Executor().
		ExecContext(ctx)
	if err != nil {
		return fmt.Errorf("update timestamp: %w", err)
	}

	return nil
}

func (r *OrmRepository) UpdateLastChecked(ctx context.Context, url string, ts time.Time) error {
	_, err := r.db.
		Update("links").
		Set(goqu.Record{"last_checked_at": ts}).
		Where(goqu.Ex{"url": url}).
		Executor().
		ExecContext(ctx)
	if err != nil {
		return fmt.Errorf("update last checked: %w", err)
	}

	return nil
}
