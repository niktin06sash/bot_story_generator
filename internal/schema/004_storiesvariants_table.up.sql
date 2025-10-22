CREATE TABLE storiesVariants(
    storyID    PRIMARY KEY,
    data    JSONB NOT NULL,
    CONSTRAINT fk_story
        FOREIGN KEY (storyID)
        REFERENCES stories(ID)
        ON DELETE CASCADE,
);