-- 脚本功能：创建用户注册使用的账号表及状态、角色约束。
-- 启动命令：在 backend 目录执行 mysql -h 127.0.0.1 -u app -p pulseframe < migrations/001_create_users.sql。
CREATE TABLE users (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    username VARCHAR(24) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    status TINYINT NOT NULL DEFAULT 1,
    role TINYINT NOT NULL DEFAULT 0,
    CONSTRAINT uq_users_username UNIQUE (username),
    CONSTRAINT chk_users_status CHECK (status IN (0, 1)),
    CONSTRAINT chk_users_role CHECK (role IN (0, 1))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;
