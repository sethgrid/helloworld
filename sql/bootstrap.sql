-- Drop the 'helloworld' database if it exists;
DROP DATABASE IF EXISTS helloworld;

-- Create the 'helloworld' database;
CREATE DATABASE helloworld;

-- Use the 'helloworld' database;
USE helloworld;

CREATE TABLE `users` (
  `id` BIGINT(20) UNSIGNED NOT NULL AUTO_INCREMENT,
  `email` VARCHAR(255) NOT NULL,
  `password` VARCHAR(255) NOT NULL,
  `verified_on` DATETIME NULL DEFAULT NULL,
  `created_at` DATETIME DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  primary key (`id`),
  index `email_password`(`email`, `password`),
  unique key `u_email` (`email`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

CREATE TABLE `apikeys` (
  `id` BIGINT(20) UNSIGNED NOT NULL AUTO_INCREMENT,
  `user_id` BIGINT(20) UNSIGNED NOT NULL,
  `apikey` VARCHAR(255) NOT NULL,
  `name` VARCHAR(255) NOT NULL DEFAULT '',
  `is_long_lived` TINYINT(1) NOT NULL DEFAULT 0,
  `can_manage_apikeys` TINYINT(1) NOT NULL DEFAULT 0,
  `created_at` DATETIME DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `expires_at` DATETIME NULL DEFAULT NULL,
  primary key (`id`),
  index (`user_id`),
  index (`apikey`),
  unique key `u_user_apikey_name` (`user_id`, `name`),
  unique key `u_apikey` (`apikey`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

CREATE TABLE `matchers` (
  `id` BIGINT(20) UNSIGNED NOT NULL AUTO_INCREMENT,
  `user_id` BIGINT(10) NOT NULL,
  `apikey_id` BIGINT(10) NOT NULL,
  `name` VARCHAR(255) NOT NULL,
  `declaration` TEXT NOT NULL,
  `created_at` DATETIME DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  primary key (`id`),
  index `uid` (`user_id`),
  index `aid` (`apikey_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

CREATE TABLE `activity_log` (
    `id` BIGINT(20) UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    `type` VARCHAR(255) NOT NULL,
    `user_id` BIGINT NOT NULL,
    `message` VARCHAR(255) NOT NULL,
    `request_path` VARCHAR(255),
    `request_verb` VARCHAR(6),
    `matcher_id` BIGINT NOT NULL,
    `apikey_id` BIGINT NOT NULL,
    `is_public` TINYINT(1) NOT NULL DEFAULT 1,
    `created_at` DATETIME DEFAULT CURRENT_TIMESTAMP
);