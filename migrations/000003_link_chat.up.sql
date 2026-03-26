CREATE TABLE link_chat (
    chat_id BIGINT NOT NULL,
    link_id INT NOT NULL,

    FOREIGN KEY (chat_id) REFERENCES chats(id) ON DELETE CASCADE,

    FOREIGN KEY (link_id) REFERENCES links(id) ON DELETE CASCADE,

    PRIMARY KEY (chat_id, link_id)
);