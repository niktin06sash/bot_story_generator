CREATE TABLE stories (
    ID   SERIAL PRIMARY KEY,
    userID     BIGINT NOT NULL,
    data       JSONB,
    createdAt  TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    isActive   BOOLEAN DEFAULT TRUE,

    CONSTRAINT fk_user
        FOREIGN KEY (userID) 
        REFERENCES users(ID)
        ON DELETE CASCADE
);
CREATE INDEX idx_stories_userID ON stories (userID);
CREATE INDEX idx_stories_isActive ON stories (isActive) WHERE isActive = TRUE;