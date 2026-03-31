CREATE TABLE link_tags (
    chat_id BIGINT NOT NULL,
    link_id INT NOT NULL,
    tag_id INT NOT NULL,

    FOREIGN KEY (chat_id, link_id) REFERENCES link_chat(chat_id, link_id) ON DELETE CASCADE,

    FOREIGN KEY (tag_id) REFERENCES tags(id) ON DELETE CASCADE,

    PRIMARY KEY (chat_id, link_id, tag_id)
);