-- Create the database
CREATE DATABASE loxilb_db;

-- Use the database
USE loxilb_db;

-- Create the users table
CREATE TABLE users (
    id INT AUTO_INCREMENT PRIMARY KEY,
    username VARCHAR(255) NOT NULL,
    password VARCHAR(255) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    role VARCHAR(255) NOT NULL DEFAULT 'viewer'
);

-- Create the token table
CREATE TABLE token (
    id INT AUTO_INCREMENT PRIMARY KEY,
    token_value VARCHAR(512) NOT NULL,
    username VARCHAR(255) NOT NULL,
    expires_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    role VARCHAR(255) NOT NULL
);
