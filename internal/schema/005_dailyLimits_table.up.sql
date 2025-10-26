CREATE TABLE dailyLimits (
    userID      BIGINT NOT NULL,
    date        DATE NOT NULL DEFAULT CURRENT_DATE,
    count    INT NOT NULL DEFAULT 1,
    limitCount  INT NOT NULL DEFAULT 20, 
    PRIMARY KEY (userID, date),
    CONSTRAINT fk_user
        FOREIGN KEY (userID)
        REFERENCES users(ID)
        ON DELETE CASCADE
);