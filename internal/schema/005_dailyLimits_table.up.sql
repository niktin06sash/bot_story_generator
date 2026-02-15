CREATE TABLE dailyLimits (
    userID      BIGINT NOT NULL,
    date        DATE NOT NULL DEFAULT CURRENT_DATE,
    count       INT NOT NULL,
    limitCount  INT NOT NULL,
    PRIMARY KEY (userID, date),
    CONSTRAINT fk_user
        FOREIGN KEY (userID)
        REFERENCES users(ID)
        ON DELETE CASCADE
);