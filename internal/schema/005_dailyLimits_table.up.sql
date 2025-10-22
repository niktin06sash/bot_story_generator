CREATE TABLE dailyLimits (
    userID      BIGINT NOT NULL,
    date        DATE NOT NULL DEFAULT CURRENT_DATE,
    msgCount    INT NOT NULL DEFAULT 0,
    dailyLimit  INT NOT NULL DEFAULT 20, 
    PRIMARY KEY (userID, date),
    CONSTRAINT fk_user
        FOREIGN KEY (userID)
        REFERENCES users(ID)
        ON DELETE CASCADE
);
CREATE INDEX idx_dailyLimits_date ON dailyLimits (date);