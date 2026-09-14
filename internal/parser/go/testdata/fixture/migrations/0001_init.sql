CREATE TABLE users (
    id INT PRIMARY KEY,
    name TEXT,
    email TEXT
);

CREATE TABLE orders (
    id INT PRIMARY KEY,
    user_id INT,
    total INT,
    FOREIGN KEY (user_id) REFERENCES users(id)
);
