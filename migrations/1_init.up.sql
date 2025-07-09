CREATE TABLE IF NOT EXISTS users (
  id uuid PRIMARY KEY,
  telegram_id VARCHAR(255) UNIQUE,
  username VARCHAR(50) UNIQUE NOT NULL,
  email VARCHAR(255) UNIQUE,
  public_address VARCHAR(44) UNIQUE,
  image_url VARCHAR(100),
  bg_url VARCHAR(100),
  x_url VARCHAR(100),
  instagram_url VARCHAR(100),
  youtube_url VARCHAR(100),
  telegram_url VARCHAR(100),
  discord_url VARCHAR(100),
  website VARCHAR(100),
  bio TEXT,
  two_fa BOOLEAN DEFAULT false,
  two_fa_secret VARCHAR(64),
  role INTEGER DEFAULT 0,
  is_premium BOOLEAN NOT NULL DEFAULT false,
  level INT NOT NULL DEFAULT 0,
  current_xp INT NOT NULL DEFAULT 0,
  balance INT NOT NULL DEFAULT 0 check (balance >= 0),
  referral_token VARCHAR(32),
  referral_count INT NOT NULL DEFAULT 0,
  referral_income_usdc DECIMAL(15, 9) NULL DEFAULT 0,
  referral_income_dp INTEGER NOT NULL DEFAULT 0,
  daily_reward_streak INTEGER NOT NULL DEFAULT 1,
  last_completed_streak TIMESTAMP WITHOUT TIME ZONE DEFAULT CURRENT_TIMESTAMP,
  created_at TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
  usdc_autoswap BOOLEAN NOT NULL DEFAULT false,
  sol_autoswap BOOLEAN NOT NULL DEFAULT false
);
CREATE TABLE IF NOT EXISTS duel_topics (
  id SERIAL PRIMARY KEY,
  name VARCHAR(32) NOT NULL UNIQUE,
  image_url VARCHAR(100)
);
CREATE TABLE IF NOT EXISTS duel_subtopics (
  id SERIAL PRIMARY KEY,
  topic_id INTEGER REFERENCES duel_topics(id) ON UPDATE CASCADE NOT NULL,
  name VARCHAR(32) NOT NULL UNIQUE,
  image_url VARCHAR(100)
);
CREATE TABLE IF NOT EXISTS duel_types (
  id SERIAL PRIMARY KEY,
  type VARCHAR(32) NOT NULL UNIQUE
);
CREATE TABLE IF NOT EXISTS duels (
  id uuid PRIMARY KEY,
  owner_id uuid REFERENCES users(id) ON UPDATE CASCADE ON DELETE CASCADE NOT NULL,
  room_number int8 UNIQUE CHECK (
    room_number > 0
    AND room_number <= 4294967295
  ),
  -- type uint32
  payment_type SMALLINT NOT NULL DEFAULT 0,
  players_count INTEGER NOT NULL DEFAULT 0,
  refunded_players_count INTEGER NOT NULL DEFAULT 0,
  winners_count INTEGER NOT NULL DEFAULT 0,
  username VARCHAR(17) NOT NULL,
  status integer NOT NULL DEFAULT 0,
  image_url TEXT default '',
  bg_url TEXT default '',
  topic VARCHAR(32) NOT NULL,
  subtopic VARCHAR(32) REFERENCES duel_subtopics(name) ON UPDATE CASCADE,
  entities INTEGER [] NOT NULL DEFAULT '{}',
  duel_type VARCHAR(32) REFERENCES duel_types(type) ON UPDATE CASCADE,
  question TEXT,
  source_of_truth TEXT,
  deadline TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
  event_date TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
  duel_price INTEGER NOT NULL,
  commission INTEGER NOT NULL,
  duel_info json,
  final_result INTEGER DEFAULT NULL,
  resolved_by uuid REFERENCES users(id) ON UPDATE CASCADE ON DELETE CASCADE,
  approved_by uuid REFERENCES users(id) ON UPDATE CASCADE ON DELETE CASCADE,
  cancellation_reason text,
  tournament_id uuid,
  created_at TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS tournaments (
  id UUID PRIMARY KEY,
  owner_id uuid REFERENCES users(id) NOT NULL,
  name TEXT NOT NULL,
  reward_pool_dp INT NOT NULL DEFAULT 0,
  reward_pool_usdc INT NOT NULL DEFAULT 0,
  description TEXT,
  author TEXT NOT NULL,
  image_url VARCHAR(255),
  bg_url VARCHAR(255),
  bg_card_url VARCHAR(255),
  x_url VARCHAR(255),
  instagram_url VARCHAR(255),
  youtube_url VARCHAR(255),
  url TEXT UNIQUE NOT NULL,
  players_count INT NOT NULL DEFAULT 0,
  start_date TIMESTAMP NOT NULL,
  finish_date TIMESTAMP NOT NULL,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS tournament_participants (
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  tournament_id UUID NOT NULL REFERENCES tournaments(id) ON DELETE CASCADE,
  CONSTRAINT unique_tournament_participant UNIQUE (user_id, tournament_id)
);
CREATE TABLE IF NOT EXISTS tournament_rewards (
  tournament_id UUID NOT NULL REFERENCES tournaments(id) ON DELETE CASCADE,
  from_rank INT NOT NULL,
  to_rank INT NOT NULL,
  dp_amount DOUBLE PRECISION NOT NULL,
  usdc_amount DOUBLE PRECISION NOT NULL
);
CREATE TABLE IF NOT EXISTS tournament_leaderboards (
  tournament_id UUID NOT NULL REFERENCES tournaments(id) ON DELETE CASCADE,
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  username varchar(50) NOT NULL,
  payment_type SMALLINT NOT NULL DEFAULT 0,
  total_duels INTEGER NOT NULL DEFAULT 0,
  victories INTEGER NOT NULL DEFAULT 0,
  spent INTEGER NOT NULL DEFAULT 0,
  earned DECIMAL(15, 9) DEFAULT 0 NOT NULL,
  pnl DECIMAL(15, 9) DEFAULT 0 NOT NULL,
  last_predicted_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  CONSTRAINT unique_tournament_user UNIQUE (tournament_id, user_id, payment_type)
);
CREATE TABLE IF NOT EXISTS tournament_offers (
  id uuid PRIMARY KEY,
  name VARCHAR(127) NOT NULL,
  contact VARCHAR(127) NOT NULL,
  project_link VARCHAR(127) NOT NULL,
  description TEXT,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS duel_entity_types (
  id SERIAL PRIMARY KEY,
  type VARCHAR(32) NOT NULL UNIQUE
);
CREATE TABLE IF NOT EXISTS duel_entities (
  id SERIAL PRIMARY KEY,
  subtopic_id INTEGER REFERENCES duel_subtopics(id) ON UPDATE CASCADE NOT NULL,
  name VARCHAR(64) NOT NULL UNIQUE,
  entity_type INTEGER REFERENCES duel_entity_types(id) ON UPDATE CASCADE NOT NULL,
  image_url VARCHAR(100),
  constraint unique_entity_name UNIQUE (subtopic_id, name)
);
CREATE TABLE IF NOT EXISTS players (
  id uuid PRIMARY KEY,
  user_id uuid REFERENCES users(id) ON UPDATE CASCADE ON DELETE CASCADE NOT NULL,
  duel_id uuid REFERENCES duels(id) NOT NULL,
  answer INTEGER NOT NULL,
  is_winner boolean DEFAULT FALSE NOT NULL,
  win_amount DECIMAL(15, 9) DEFAULT 0 NOT NULL,
  final_status SMALLINT NOT NULL DEFAULT 0,
  created_at TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);
-- todo drop wallets
CREATE TABLE IF NOT EXISTS wallets (
  user_id UUID REFERENCES users(id) ON UPDATE CASCADE ON DELETE CASCADE,
  address VARCHAR(44) NOT NULL UNIQUE,
  name VARCHAR(255) NOT NULL,
  created_at timestamp without time zone NOT NULL DEFAULT current_timestamp,
  updated_at timestamp without time zone NOT NULL DEFAULT current_timestamp
);
CREATE TABLE IF NOT EXISTS coins (
  id int8 PRIMARY KEY,
  rank int8 NOT NULL,
  name varchar(100) NOT NULL,
  symbol varchar(50) NOT NULL,
  slug varchar(100) NOT NULL,
  image_url varchar(200) NOT NULL
);
CREATE TABLE IF NOT EXISTS user_referrals (
  referrer_id UUID REFERENCES users(id) ON DELETE CASCADE NOT NULL,
  referral_id UUID REFERENCES users(id) ON DELETE CASCADE NOT NULL UNIQUE,
  income_usdc DECIMAL(15, 9) NOT NULL DEFAULT 0,
  income_dp INTEGER NOT NULL DEFAULT 0,
  referral_commission_end TIMESTAMP NOT NULl,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS tasks (
  id SERIAL PRIMARY KEY,
  name VARCHAR(64) NOT NULL,
  description TEXT,
  reward INT NOT NULL,
  currency SMALLINT NOT NULL,
  -- currency enum type
  xp INT NOT NULL DEFAULT 0,
  completion_limit INT NOT NULL DEFAULT 0,
  -- number of times user can get a reward in the limit period. 0 - no limits
  completion_limit_period INT NOT NULL DEFAULT 0,
  -- limit period in days. 0 - no limits
  task_notes VARCHAR(100),
  link VARCHAR(255),
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS completed_tasks (
  user_id UUID NOT NULL REFERENCES users(id),
  task_id INT NOT NULL REFERENCES tasks(id),
  reward_claimed INT NOT NULL DEFAULT 0,
  overall_count INT NOT NULL DEFAULT 1, -- counter of all task completions
  completion_count INT NOT NULL DEFAULT 1, -- counter of task completion in the task limit period
  reward_multiplier DOUBLE PRECISION NOT NULL DEFAULT 1,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (user_id, task_id)
);

CREATE TABLE IF NOT EXISTS completed_tasks_rewards (
  user_id UUID NOT NULL REFERENCES users(id),
  task_id INT NOT NULL REFERENCES tasks(id),
  overall_reward INT NOT NULL DEFAULT 0,
  overall_xp INT NOT NULL DEFAULT 0,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (user_id, task_id)
);

CREATE TABLE IF NOT EXISTS tg_channel_referrals (
  telegram_link VARCHAR(37) NOT NULL UNIQUE,
  referral_token VARCHAR(8) NOT NULL UNIQUE,
  referral_count INT NOT NULL DEFAULT 0,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS moderator_stats (
  moderator_id UUID REFERENCES users(id) ON UPDATE CASCADE ON DELETE CASCADE,
  duel_id UUID REFERENCES duels(id) ON UPDATE CASCADE ON DELETE CASCADE,
  action_type VARCHAR(32) NOT NULL,
  creation_pay DOUBLE PRECISION NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS duck_points_transactions (
  sender_address VARCHAR(44) NOT NULL REFERENCES users(public_address) ON UPDATE CASCADE ON DELETE CASCADE,
  recipient_address VARCHAR(44) NOT NULL REFERENCES users(public_address) ON UPDATE CASCADE ON DELETE CASCADE,
  amount INTEGER NOT NULL check(amount > 0),
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS transactions (
  signature VARCHAR(88) PRIMARY KEY,
  tx_type SMALLINT NOT NULL CHECK(tx_type IN (1, 2, 3, 4))
);
CREATE TABLE IF NOT EXISTS solana_tokens (
  mint VARCHAR(44) PRIMARY KEY,
  name VARCHAR(128) NOT NULL,
  symbol VARCHAR(10) NOT NULL,
  decimals SMALLINT NOT NULL,
  image_url TEXT,
  daily_volume DOUBLE PRECISION NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS wallet_tokens (
  user_id uuid REFERENCES users(id) ON UPDATE CASCADE ON DELETE CASCADE NOT NULL,
  mint VARCHAR(44) NOT NULL,
  name VARCHAR(128) NOT NULL,
  symbol VARCHAR(10) NOT NULL,
  image_url TEXT,
  is_visible BOOLEAN NOT NULL DEFAULT FALSE,
  is_swappable BOOLEAN NOT NULL DEFAULT FALSE,
  CONSTRAINT unique_user_per_token UNIQUE (user_id, mint)
);

-- TODO: optimize performance, separating leaderboards to several materialized views
CREATE MATERIALIZED VIEW IF NOT EXISTS leaderboard AS WITH played_duels AS (
  SELECT
    players.user_id,
    players.duel_id,
    players.is_winner,
    players.win_amount,
    duels.topic,
    duels.duel_price,
    duels.payment_type,
    CASE
      WHEN players.user_id = duels.owner_id THEN (
        (
          duels.players_count * duels.duel_price * duels.commission / 100
        ) / 2
      )
      ELSE 0
    END AS comission_earned
  FROM
    players
    INNER JOIN duels ON players.duel_id = duels.id
  WHERE
    duels.status = 5
)
SELECT
  u.id AS user_id,
  u.username,
  pd.topic,
  pd.payment_type,
  COUNT(pd.duel_id) AS total_duels,
  COUNT(
    CASE
      WHEN pd.is_winner = true THEN 1
    END
  ) AS victories,
  SUM(pd.duel_price) AS spent,
  SUM(pd.win_amount) AS earned,
  SUM(pd.win_amount + pd.comission_earned) - SUM(pd.duel_price) AS pnl
FROM
  users AS u
  LEFT JOIN played_duels AS pd ON u.id = pd.user_id
GROUP BY
  pd.payment_type,
  u.id,
  u.username,
  pd.topic
HAVING
  COUNT(pd.duel_id) > 0
ORDER BY
  pnl DESC;
CREATE MATERIALIZED VIEW IF NOT EXISTS tournament_leaderboard AS WITH played_duels AS (
    SELECT
      players.user_id,
      players.duel_id,
      players.is_winner,
      players.win_amount,
      duels.topic,
      duels.duel_price,
      duels.payment_type,
      players.created_at
    FROM
      players
      INNER JOIN duels ON players.duel_id = duels.id
    WHERE
      players.created_at >= '2025-04-01T00:00:00Z'
      AND duels.status = 5
      AND duels.tournament_id = '885fa908-ed18-4dde-a376-32ff921b8783'
  )
SELECT
  u.id AS user_id,
  u.username,
  pd.topic,
  pd.payment_type,
  COUNT(pd.duel_id) AS total_duels,
  COUNT(
    CASE
      WHEN pd.is_winner = true THEN 1
    END
  ) AS victories,
  SUM(pd.duel_price) AS spent,
  SUM(pd.win_amount) AS earned,
  SUM(pd.win_amount) - SUM(pd.duel_price) AS pnl,
  MAX(pd.created_at) AS last_predicted_at
FROM
  users AS u
  LEFT JOIN played_duels AS pd ON u.id = pd.user_id
GROUP BY
  pd.payment_type,
  u.id,
  u.username,
  pd.topic
HAVING
  COUNT(pd.duel_id) > 0
ORDER BY
  pnl DESC,
  last_predicted_at ASC;
CREATE MATERIALIZED VIEW IF NOT EXISTS tournament_leaderboard_dd AS WITH played_duels AS (
    SELECT
      players.user_id,
      players.duel_id,
      players.is_winner,
      players.win_amount,
      duels.topic,
      duels.duel_price,
      duels.payment_type,
      players.created_at
    FROM
      players
      INNER JOIN duels ON players.duel_id = duels.id
    WHERE
      duels.status = 5
      AND duels.tournament_id = 'd11d0ee6-e74d-4844-b22f-3a4962e57f9e'
  )
SELECT
  u.id AS user_id,
  u.username,
  pd.topic,
  pd.payment_type,
  COUNT(pd.duel_id) AS total_duels,
  COUNT(
    CASE
      WHEN pd.is_winner = true THEN 1
    END
  ) AS victories,
  SUM(pd.duel_price) AS spent,
  SUM(pd.win_amount) AS earned,
  SUM(pd.win_amount) - SUM(pd.duel_price) AS pnl,
  MAX(pd.created_at) AS last_predicted_at
FROM
  users AS u
  LEFT JOIN played_duels AS pd ON u.id = pd.user_id
GROUP BY
  pd.payment_type,
  u.id,
  u.username,
  pd.topic
HAVING
  COUNT(pd.duel_id) > 0
ORDER BY
  pnl DESC,
  last_predicted_at ASC;