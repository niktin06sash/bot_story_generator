CREATE TABLE subscriptions (
    chargeId TEXT PRIMARY KEY,
    userID BIGINT NOT NULL,
    type TEXT NOT NULL,                 
    startDate TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    endDate TIMESTAMP WITH TIME ZONE NOT NULL,
    isAutoRenewal BOOLEAN DEFAULT TRUE,
    payload TEXT NOT NULL,
    CONSTRAINT fk_user
        FOREIGN KEY (userID)
        REFERENCES users(ID)
        ON DELETE CASCADE
);
CREATE INDEX idx_subscriptions_user_id ON subscriptions (userID);
CREATE INDEX idx_subscriptions_end_date ON subscriptions (endDate);
