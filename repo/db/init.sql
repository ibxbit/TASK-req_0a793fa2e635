CREATE DATABASE IF NOT EXISTS helios_db
  CHARACTER SET utf8mb4
  COLLATE utf8mb4_unicode_ci;

USE helios_db;

SET FOREIGN_KEY_CHECKS = 0;

-- =========================================================
-- USERS & ROLES (RBAC)
-- =========================================================
CREATE TABLE IF NOT EXISTS roles (
  id            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  name          VARCHAR(64) NOT NULL,
  description   VARCHAR(255) DEFAULT NULL,
  permissions   JSON DEFAULT NULL,
  created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uq_roles_name (name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS users (
  id              BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  username        VARCHAR(64) NOT NULL,
  password_hash   VARCHAR(255) NOT NULL,
  email           VARCHAR(128) DEFAULT NULL,
  role_id         BIGINT UNSIGNED NOT NULL,
  status          ENUM('active','disabled','locked') NOT NULL DEFAULT 'active',
  last_login_at   DATETIME DEFAULT NULL,
  created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uq_users_username (username),
  UNIQUE KEY uq_users_email (email),
  KEY idx_users_role (role_id),
  CONSTRAINT fk_users_role FOREIGN KEY (role_id) REFERENCES roles(id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- =========================================================
-- CONTENT
-- =========================================================
CREATE TABLE IF NOT EXISTS dynasties (
  id           BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  name         VARCHAR(64) NOT NULL,
  start_year   INT DEFAULT NULL,
  end_year     INT DEFAULT NULL,
  description  TEXT DEFAULT NULL,
  created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uq_dynasties_name (name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS authors (
  id            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  name          VARCHAR(128) NOT NULL,
  alt_names     VARCHAR(255) DEFAULT NULL,
  dynasty_id    BIGINT UNSIGNED DEFAULT NULL,
  birth_year    INT DEFAULT NULL,
  death_year    INT DEFAULT NULL,
  biography     TEXT DEFAULT NULL,
  created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  KEY idx_authors_dynasty (dynasty_id),
  KEY idx_authors_name (name),
  CONSTRAINT fk_authors_dynasty FOREIGN KEY (dynasty_id) REFERENCES dynasties(id) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS genres (
  id           BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  name         VARCHAR(64) NOT NULL,
  kind         ENUM('genre','tag') NOT NULL DEFAULT 'genre',
  parent_id    BIGINT UNSIGNED DEFAULT NULL,
  description  VARCHAR(255) DEFAULT NULL,
  created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uq_genres_name_kind (name, kind),
  KEY idx_genres_parent (parent_id),
  CONSTRAINT fk_genres_parent FOREIGN KEY (parent_id) REFERENCES genres(id) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS meter_patterns (
  id            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  name          VARCHAR(128) NOT NULL,
  pattern_type  ENUM('meter','tune','ci_pai','qu_pai') NOT NULL DEFAULT 'meter',
  rhythm        TEXT DEFAULT NULL,
  rhyme_scheme  VARCHAR(255) DEFAULT NULL,
  tonal_pattern TEXT DEFAULT NULL,
  description   TEXT DEFAULT NULL,
  created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uq_meter_name_type (name, pattern_type)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS poems (
  id                BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  title             VARCHAR(255) NOT NULL,
  author_id         BIGINT UNSIGNED DEFAULT NULL,
  dynasty_id        BIGINT UNSIGNED DEFAULT NULL,
  meter_pattern_id  BIGINT UNSIGNED DEFAULT NULL,
  body              MEDIUMTEXT NOT NULL,
  preface           TEXT DEFAULT NULL,
  translation       MEDIUMTEXT DEFAULT NULL,
  source            VARCHAR(255) DEFAULT NULL,
  status            ENUM('draft','in_review','published','archived') NOT NULL DEFAULT 'draft',
  version           INT UNSIGNED NOT NULL DEFAULT 1,
  created_by        BIGINT UNSIGNED DEFAULT NULL,
  updated_by        BIGINT UNSIGNED DEFAULT NULL,
  created_at        DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at        DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  KEY idx_poems_author (author_id),
  KEY idx_poems_dynasty (dynasty_id),
  KEY idx_poems_meter (meter_pattern_id),
  KEY idx_poems_status (status),
  KEY idx_poems_title (title),
  CONSTRAINT fk_poems_author  FOREIGN KEY (author_id)        REFERENCES authors(id)         ON DELETE SET NULL,
  CONSTRAINT fk_poems_dynasty FOREIGN KEY (dynasty_id)       REFERENCES dynasties(id)       ON DELETE SET NULL,
  CONSTRAINT fk_poems_meter   FOREIGN KEY (meter_pattern_id) REFERENCES meter_patterns(id)  ON DELETE SET NULL,
  CONSTRAINT fk_poems_creator FOREIGN KEY (created_by)       REFERENCES users(id)           ON DELETE SET NULL,
  CONSTRAINT fk_poems_updater FOREIGN KEY (updated_by)       REFERENCES users(id)           ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS poem_genres (
  poem_id     BIGINT UNSIGNED NOT NULL,
  genre_id    BIGINT UNSIGNED NOT NULL,
  created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (poem_id, genre_id),
  KEY idx_pg_genre (genre_id),
  CONSTRAINT fk_pg_poem  FOREIGN KEY (poem_id)  REFERENCES poems(id)  ON DELETE CASCADE,
  CONSTRAINT fk_pg_genre FOREIGN KEY (genre_id) REFERENCES genres(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS excerpts (
  id             BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  poem_id        BIGINT UNSIGNED NOT NULL,
  start_offset   INT UNSIGNED NOT NULL,
  end_offset     INT UNSIGNED NOT NULL,
  excerpt_text   TEXT NOT NULL,
  annotation     TEXT DEFAULT NULL,
  annotation_type ENUM('note','commentary','translation','reference') NOT NULL DEFAULT 'note',
  author_id      BIGINT UNSIGNED DEFAULT NULL,
  created_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  KEY idx_excerpts_poem (poem_id),
  KEY idx_excerpts_author (author_id),
  CONSTRAINT fk_excerpts_poem   FOREIGN KEY (poem_id)   REFERENCES poems(id) ON DELETE CASCADE,
  CONSTRAINT fk_excerpts_author FOREIGN KEY (author_id) REFERENCES users(id) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- =========================================================
-- PRICING
-- =========================================================
CREATE TABLE IF NOT EXISTS member_tiers (
  id             BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  name           VARCHAR(64) NOT NULL,
  level          INT UNSIGNED NOT NULL DEFAULT 0,
  monthly_price  DECIMAL(10,2) NOT NULL DEFAULT 0.00,
  yearly_price   DECIMAL(10,2) NOT NULL DEFAULT 0.00,
  benefits       JSON DEFAULT NULL,
  created_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uq_tiers_name (name),
  UNIQUE KEY uq_tiers_level (level)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS campaigns (
  id              BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  name            VARCHAR(128) NOT NULL,
  description     TEXT DEFAULT NULL,
  campaign_type   ENUM('standard','flash_sale','group_buy') NOT NULL DEFAULT 'standard',
  discount_type   ENUM('percentage','fixed') NOT NULL DEFAULT 'percentage',
  discount_value  DECIMAL(10,2) NOT NULL DEFAULT 0.00,
  min_group_size  INT UNSIGNED DEFAULT NULL,
  status          ENUM('draft','active','paused','ended') NOT NULL DEFAULT 'draft',
  starts_at       DATETIME DEFAULT NULL,
  ends_at         DATETIME DEFAULT NULL,
  created_by      BIGINT UNSIGNED DEFAULT NULL,
  created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  KEY idx_campaigns_status (status),
  KEY idx_campaigns_window (starts_at, ends_at),
  KEY idx_campaigns_type (campaign_type),
  CONSTRAINT fk_campaigns_creator FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS pricing_rules (
  id            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  name          VARCHAR(128) NOT NULL,
  rule_type     ENUM('percentage','fixed','tiered','bundle') NOT NULL DEFAULT 'percentage',
  target_scope  ENUM('all','poem','author','dynasty','genre','tier') NOT NULL DEFAULT 'all',
  target_id     BIGINT UNSIGNED DEFAULT NULL,
  value         DECIMAL(10,2) NOT NULL DEFAULT 0.00,
  min_amount    DECIMAL(10,2) DEFAULT NULL,
  max_discount  DECIMAL(10,2) DEFAULT NULL,
  priority      INT NOT NULL DEFAULT 0,
  campaign_id   BIGINT UNSIGNED DEFAULT NULL,
  active        TINYINT(1) NOT NULL DEFAULT 1,
  starts_at     DATETIME DEFAULT NULL,
  ends_at       DATETIME DEFAULT NULL,
  created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  KEY idx_rules_campaign (campaign_id),
  KEY idx_rules_active (active),
  CONSTRAINT fk_rules_campaign FOREIGN KEY (campaign_id) REFERENCES campaigns(id) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS coupons (
  id             BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  code           VARCHAR(64) NOT NULL,
  campaign_id    BIGINT UNSIGNED DEFAULT NULL,
  discount_type  ENUM('percentage','fixed') NOT NULL DEFAULT 'percentage',
  discount_value DECIMAL(10,2) NOT NULL DEFAULT 0.00,
  min_amount     DECIMAL(10,2) DEFAULT NULL,
  usage_limit    INT UNSIGNED DEFAULT NULL,
  used_count     INT UNSIGNED NOT NULL DEFAULT 0,
  per_user_limit INT UNSIGNED DEFAULT NULL,
  starts_at      DATETIME DEFAULT NULL,
  ends_at        DATETIME DEFAULT NULL,
  status         ENUM('active','disabled','expired') NOT NULL DEFAULT 'active',
  created_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uq_coupons_code (code),
  KEY idx_coupons_campaign (campaign_id),
  KEY idx_coupons_status (status),
  CONSTRAINT fk_coupons_campaign FOREIGN KEY (campaign_id) REFERENCES campaigns(id) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS orders (
  id              BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  order_no        VARCHAR(64) NOT NULL,
  user_id         BIGINT UNSIGNED NOT NULL,
  tier_id         BIGINT UNSIGNED DEFAULT NULL,
  coupon_id       BIGINT UNSIGNED DEFAULT NULL,
  amount          DECIMAL(10,2) NOT NULL DEFAULT 0.00,
  discount_amount DECIMAL(10,2) NOT NULL DEFAULT 0.00,
  final_amount    DECIMAL(10,2) NOT NULL DEFAULT 0.00,
  currency        CHAR(3) NOT NULL DEFAULT 'CNY',
  status          ENUM('pending','paid','refunded','cancelled','failed') NOT NULL DEFAULT 'pending',
  items           JSON DEFAULT NULL,
  paid_at         DATETIME DEFAULT NULL,
  created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uq_orders_no (order_no),
  KEY idx_orders_user (user_id),
  KEY idx_orders_tier (tier_id),
  KEY idx_orders_coupon (coupon_id),
  KEY idx_orders_status (status),
  CONSTRAINT fk_orders_user   FOREIGN KEY (user_id)   REFERENCES users(id)         ON DELETE RESTRICT,
  CONSTRAINT fk_orders_tier   FOREIGN KEY (tier_id)   REFERENCES member_tiers(id)  ON DELETE SET NULL,
  CONSTRAINT fk_orders_coupon FOREIGN KEY (coupon_id) REFERENCES coupons(id)       ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS entitlements (
  id            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  user_id       BIGINT UNSIGNED NOT NULL,
  tier_id       BIGINT UNSIGNED DEFAULT NULL,
  order_id      BIGINT UNSIGNED DEFAULT NULL,
  scope         ENUM('tier','poem','bundle','feature') NOT NULL DEFAULT 'tier',
  resource_id   BIGINT UNSIGNED DEFAULT NULL,
  starts_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  expires_at    DATETIME DEFAULT NULL,
  status        ENUM('active','expired','revoked') NOT NULL DEFAULT 'active',
  created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  KEY idx_ent_user (user_id),
  KEY idx_ent_tier (tier_id),
  KEY idx_ent_order (order_id),
  KEY idx_ent_status (status),
  CONSTRAINT fk_ent_user  FOREIGN KEY (user_id)  REFERENCES users(id)        ON DELETE CASCADE,
  CONSTRAINT fk_ent_tier  FOREIGN KEY (tier_id)  REFERENCES member_tiers(id) ON DELETE SET NULL,
  CONSTRAINT fk_ent_order FOREIGN KEY (order_id) REFERENCES orders(id)       ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- =========================================================
-- FEEDBACK
-- =========================================================
CREATE TABLE IF NOT EXISTS reviews (
  id                 BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  poem_id            BIGINT UNSIGNED NOT NULL,
  user_id            BIGINT UNSIGNED NOT NULL,
  rating             TINYINT UNSIGNED NOT NULL DEFAULT 0,
  rating_accuracy    TINYINT UNSIGNED NOT NULL DEFAULT 0,
  rating_readability TINYINT UNSIGNED NOT NULL DEFAULT 0,
  rating_value       TINYINT UNSIGNED NOT NULL DEFAULT 0,
  title              VARCHAR(255) DEFAULT NULL,
  content            TEXT DEFAULT NULL,
  status             ENUM('pending','approved','rejected','hidden') NOT NULL DEFAULT 'pending',
  created_at         DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at         DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  KEY idx_reviews_poem (poem_id),
  KEY idx_reviews_user (user_id),
  KEY idx_reviews_status (status),
  CONSTRAINT fk_reviews_poem FOREIGN KEY (poem_id) REFERENCES poems(id) ON DELETE CASCADE,
  CONSTRAINT fk_reviews_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS arbitration_status (
  id             BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  code           VARCHAR(64) NOT NULL,
  label          VARCHAR(128) NOT NULL,
  is_terminal    TINYINT(1) NOT NULL DEFAULT 0,
  sort_order     INT NOT NULL DEFAULT 0,
  created_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uq_arb_code (code)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS complaints (
  id                BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  complainant_id    BIGINT UNSIGNED NOT NULL,
  target_type       ENUM('poem','review','user','order','other') NOT NULL,
  target_id         BIGINT UNSIGNED DEFAULT NULL,
  subject           VARCHAR(255) NOT NULL,
  notes_encrypted   VARBINARY(8192) DEFAULT NULL,
  encryption_scheme VARCHAR(32) DEFAULT NULL,
  arbitration_id    BIGINT UNSIGNED DEFAULT NULL,
  arbitrator_id     BIGINT UNSIGNED DEFAULT NULL,
  resolution        TEXT DEFAULT NULL,
  resolved_at       DATETIME DEFAULT NULL,
  created_at        DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at        DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  KEY idx_complaints_complainant (complainant_id),
  KEY idx_complaints_arbitration (arbitration_id),
  KEY idx_complaints_arbitrator (arbitrator_id),
  KEY idx_complaints_target (target_type, target_id),
  CONSTRAINT fk_complaints_user    FOREIGN KEY (complainant_id) REFERENCES users(id)              ON DELETE CASCADE,
  CONSTRAINT fk_complaints_arb     FOREIGN KEY (arbitration_id) REFERENCES arbitration_status(id) ON DELETE SET NULL,
  CONSTRAINT fk_complaints_arbiter FOREIGN KEY (arbitrator_id)  REFERENCES users(id)              ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- =========================================================
-- CRAWLING
-- =========================================================
CREATE TABLE IF NOT EXISTS crawl_nodes (
  id            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  node_name     VARCHAR(128) NOT NULL,
  host          VARCHAR(255) NOT NULL,
  port          INT UNSIGNED DEFAULT NULL,
  status        ENUM('online','offline','degraded','maintenance') NOT NULL DEFAULT 'offline',
  capabilities  JSON DEFAULT NULL,
  last_heartbeat_at DATETIME DEFAULT NULL,
  created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uq_nodes_name (node_name),
  KEY idx_nodes_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS crawl_jobs (
  id               BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  job_name         VARCHAR(128) NOT NULL,
  node_id          BIGINT UNSIGNED DEFAULT NULL,
  source_url       VARCHAR(1024) DEFAULT NULL,
  config           JSON DEFAULT NULL,
  checkpoint       JSON DEFAULT NULL,
  status           ENUM('queued','running','paused','completed','failed','cancelled') NOT NULL DEFAULT 'queued',
  priority         INT NOT NULL DEFAULT 0,
  attempts         INT UNSIGNED NOT NULL DEFAULT 0,
  max_attempts     INT UNSIGNED NOT NULL DEFAULT 5,
  next_attempt_at  DATETIME DEFAULT NULL,
  last_error       TEXT DEFAULT NULL,
  pages_fetched    INT UNSIGNED NOT NULL DEFAULT 0,
  -- Per-day quota enforcement. `pages_fetched_today` is reset to 0 whenever
  -- the worker encounters a new `quota_date`; `daily_quota` is the cap for
  -- that day. When pages_fetched_today >= daily_quota, the worker pauses the
  -- job until the next UTC day rolls over.
  pages_fetched_today INT UNSIGNED NOT NULL DEFAULT 0,
  quota_date          DATE DEFAULT NULL,
  daily_quota      INT UNSIGNED NOT NULL DEFAULT 10000,
  scheduled_at     DATETIME DEFAULT NULL,
  started_at       DATETIME DEFAULT NULL,
  finished_at      DATETIME DEFAULT NULL,
  created_by       BIGINT UNSIGNED DEFAULT NULL,
  created_at       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  KEY idx_jobs_node (node_id),
  KEY idx_jobs_status (status),
  KEY idx_jobs_creator (created_by),
  KEY idx_jobs_retry (status, next_attempt_at),
  CONSTRAINT fk_jobs_node    FOREIGN KEY (node_id)    REFERENCES crawl_nodes(id) ON DELETE SET NULL,
  CONSTRAINT fk_jobs_creator FOREIGN KEY (created_by) REFERENCES users(id)       ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS crawl_logs (
  id           BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  job_id       BIGINT UNSIGNED NOT NULL,
  node_id      BIGINT UNSIGNED DEFAULT NULL,
  level        ENUM('debug','info','warn','error','fatal') NOT NULL DEFAULT 'info',
  message      TEXT NOT NULL,
  context      JSON DEFAULT NULL,
  logged_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  KEY idx_clogs_job (job_id),
  KEY idx_clogs_node (node_id),
  KEY idx_clogs_level (level),
  KEY idx_clogs_time (logged_at),
  CONSTRAINT fk_clogs_job  FOREIGN KEY (job_id)  REFERENCES crawl_jobs(id)  ON DELETE CASCADE,
  CONSTRAINT fk_clogs_node FOREIGN KEY (node_id) REFERENCES crawl_nodes(id) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS crawl_metrics (
  id              BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  job_id          BIGINT UNSIGNED DEFAULT NULL,
  node_id         BIGINT UNSIGNED DEFAULT NULL,
  metric_name     VARCHAR(128) NOT NULL,
  metric_value    DOUBLE NOT NULL DEFAULT 0,
  unit            VARCHAR(32) DEFAULT NULL,
  tags            JSON DEFAULT NULL,
  recorded_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  KEY idx_cmetrics_job (job_id),
  KEY idx_cmetrics_node (node_id),
  KEY idx_cmetrics_name_time (metric_name, recorded_at),
  CONSTRAINT fk_cmetrics_job  FOREIGN KEY (job_id)  REFERENCES crawl_jobs(id)  ON DELETE CASCADE,
  CONSTRAINT fk_cmetrics_node FOREIGN KEY (node_id) REFERENCES crawl_nodes(id) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- =========================================================
-- AUDIT (rollback support — retain 30 days)
-- =========================================================
CREATE TABLE IF NOT EXISTS audit_logs (
  id                 BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  actor_id           BIGINT UNSIGNED DEFAULT NULL,
  actor_role         VARCHAR(64) DEFAULT NULL,
  action             ENUM('create','update','delete','restore','login','logout','other') NOT NULL,
  entity_type        VARCHAR(64) NOT NULL,
  entity_id          BIGINT UNSIGNED DEFAULT NULL,
  before_json        JSON DEFAULT NULL,
  after_json         JSON DEFAULT NULL,
  ip_address         VARCHAR(45) DEFAULT NULL,
  user_agent         VARCHAR(255) DEFAULT NULL,
  reason             VARCHAR(255) DEFAULT NULL,
  expires_at         DATETIME NOT NULL,
  batch_id           VARCHAR(64) DEFAULT NULL,
  approval_status    ENUM('not_required','pending','approved','rejected','reverted') NOT NULL DEFAULT 'not_required',
  approval_deadline  DATETIME DEFAULT NULL,
  approved_by        BIGINT UNSIGNED DEFAULT NULL,
  approved_at        DATETIME DEFAULT NULL,
  created_at         DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at         DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  KEY idx_audit_actor (actor_id),
  KEY idx_audit_entity (entity_type, entity_id),
  KEY idx_audit_action (action),
  KEY idx_audit_expires (expires_at),
  KEY idx_audit_created (created_at),
  KEY idx_audit_batch (batch_id),
  KEY idx_audit_approval (approval_status, approval_deadline),
  CONSTRAINT fk_audit_actor    FOREIGN KEY (actor_id)    REFERENCES users(id) ON DELETE SET NULL,
  CONSTRAINT fk_audit_approver FOREIGN KEY (approved_by) REFERENCES users(id) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS system_settings (
  setting_key    VARCHAR(64) NOT NULL,
  setting_value  VARCHAR(255) NOT NULL,
  description    VARCHAR(255) DEFAULT NULL,
  created_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (setting_key)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS idempotency_keys (
  id             BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  idem_key       VARCHAR(128) NOT NULL,
  user_id        BIGINT UNSIGNED NOT NULL DEFAULT 0,
  method         VARCHAR(10) NOT NULL,
  path           VARCHAR(255) NOT NULL,
  status_code    INT NOT NULL DEFAULT 0,
  response_body  MEDIUMTEXT DEFAULT NULL,
  expires_at     DATETIME NOT NULL,
  created_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uq_idem (idem_key, user_id, method, path),
  KEY idx_idem_expires (expires_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- =========================================================
-- SYSTEM
-- =========================================================
CREATE TABLE IF NOT EXISTS query_history (
  id            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  user_id       BIGINT UNSIGNED DEFAULT NULL,
  query_text    TEXT NOT NULL,
  filters       JSON DEFAULT NULL,
  result_count  INT UNSIGNED NOT NULL DEFAULT 0,
  duration_ms   INT UNSIGNED NOT NULL DEFAULT 0,
  executed_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  KEY idx_qh_user (user_id),
  KEY idx_qh_time (executed_at),
  CONSTRAINT fk_qh_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS dictionary_terms (
  id             BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  term           VARCHAR(128) NOT NULL,
  pinyin         VARCHAR(255) DEFAULT NULL,
  definition     TEXT DEFAULT NULL,
  category       VARCHAR(64) DEFAULT NULL,
  source         VARCHAR(255) DEFAULT NULL,
  examples       JSON DEFAULT NULL,
  created_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uq_dict_term (term),
  KEY idx_dict_category (category)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS crash_reports (
  id             BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  service        VARCHAR(64) NOT NULL,
  environment    VARCHAR(32) NOT NULL DEFAULT 'production',
  error_type     VARCHAR(128) NOT NULL,
  error_message  TEXT DEFAULT NULL,
  stack_trace    MEDIUMTEXT DEFAULT NULL,
  context        JSON DEFAULT NULL,
  user_id        BIGINT UNSIGNED DEFAULT NULL,
  occurred_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  resolved       TINYINT(1) NOT NULL DEFAULT 0,
  created_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  KEY idx_crash_service (service),
  KEY idx_crash_user (user_id),
  KEY idx_crash_time (occurred_at),
  KEY idx_crash_resolved (resolved),
  CONSTRAINT fk_crash_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS performance_metrics (
  id             BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  service        VARCHAR(64) NOT NULL,
  metric_name    VARCHAR(128) NOT NULL,
  metric_value   DOUBLE NOT NULL DEFAULT 0,
  unit           VARCHAR(32) DEFAULT NULL,
  tags           JSON DEFAULT NULL,
  recorded_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  created_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  KEY idx_perf_service (service),
  KEY idx_perf_name_time (metric_name, recorded_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- =========================================================
-- SEED: Roles (RBAC)
-- =========================================================
INSERT INTO roles (name, description) VALUES
  ('administrator',    'Full system access'),
  ('content_editor',   'Create and edit content'),
  ('reviewer',         'Review and arbitrate'),
  ('marketing_manager','Manage campaigns, coupons, pricing'),
  ('crawler_operator', 'Manage crawl jobs and nodes'),
  ('member',           'Regular end-user: search, browse, submit own reviews and complaints')
ON DUPLICATE KEY UPDATE description = VALUES(description);

INSERT INTO system_settings (setting_key, setting_value, description) VALUES
  ('approval_required', 'false', 'If true, deletions and bulk edits require admin approval within 48 hours or auto-revert')
ON DUPLICATE KEY UPDATE description = VALUES(description);

INSERT INTO arbitration_status (code, label, is_terminal, sort_order) VALUES
  ('submitted',         'Submitted',           0, 10),
  ('under_review',      'Under Review',        0, 20),
  ('awaiting_evidence', 'Awaiting Evidence',   0, 30),
  ('escalated',         'Escalated',           0, 40),
  ('resolved_upheld',   'Resolved - Upheld',   1, 50),
  ('resolved_rejected', 'Resolved - Rejected', 1, 60),
  ('withdrawn',         'Withdrawn',           1, 70)
ON DUPLICATE KEY UPDATE label = VALUES(label), is_terminal = VALUES(is_terminal), sort_order = VALUES(sort_order);

SET FOREIGN_KEY_CHECKS = 1;
