CREATE TABLE subscriptions (
    payload TEXT PRIMARY KEY,
    chargeId TEXT,
    userID BIGINT NOT NULL,
    type TEXT NOT NULL,
    status TEXT NOT NULL,                 
    startDate TIMESTAMP WITH TIME ZONE,
    endDate TIMESTAMP WITH TIME ZONE,
    isAutoRenewal BOOLEAN DEFAULT TRUE,
    currency TEXT NOT NULL,
    price INT NOT NULL,
    CONSTRAINT fk_user
        FOREIGN KEY (userID)
        REFERENCES users(ID)
        ON DELETE CASCADE
);
CREATE INDEX idx_subscriptions_user_id ON subscriptions (userID);
CREATE INDEX idx_subscriptions_end_date ON subscriptions (endDate);
