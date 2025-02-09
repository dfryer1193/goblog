-- dfryer:werewolvesBlog:Create initial schema
CREATE TABLE IF NOT EXISTS posts (
    id varchar(200) PRIMARY KEY,
    title varchar(200) NOT NULL,
    createdAt timestamp NOT NULL,
    updatedAt timestamp NOT NULL,
    content text NOT NULL
);

CREATE TABLE IF NOT EXISTS comments (
    id serial PRIMARY KEY,
    author varchar(50) NOT NULL,
    createdAt timestamp NOT NULL,
    postId varchar(200) NOT NULL,
    replyTo int,
    content text NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_post_id ON comments(postId);
