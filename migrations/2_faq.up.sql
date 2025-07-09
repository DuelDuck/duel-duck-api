CREATE TABLE IF NOT EXISTS faq_anonymous_users (
    id UUID PRIMARY KEY, 
    ip varchar(40) not null,
    agent varchar(255) not null,
    created_at timestamp default timezone('UTC', now()) not null
);

CREATE TABLE IF NOT EXISTS faq_moderators (
    id serial not null constraint faq_moderators_pk primary key,
    telegram_id bigint not null unique
);

CREATE TABLE IF NOT EXISTS faq_questions (
    id SERIAL PRIMARY KEY,
    user_id UUID REFERENCES users(id) ON UPDATE CASCADE ON DELETE SET NULL,
    anonymous_user_id uuid REFERENCES faq_anonymous_users(id) ON UPDATE CASCADE ON DELETE SET NULL,
    question TEXT NOT NULL,
    answered bool NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    images TEXT []
    
    CHECK (
        (user_id IS NOT NULL AND anonymous_user_id IS NULL)
        OR
        (user_id IS NULL AND anonymous_user_id IS NOT NULL)
    )
);

CREATE TABLE IF NOT EXISTS faq_answers (
    id SERIAL PRIMARY KEY,
    question_id INT REFERENCES faq_questions(id) ON UPDATE CASCADE ON DELETE CASCADE,
    moderator_id INT REFERENCES faq_moderators(id) ON UPDATE CASCADE ON DELETE SET NULL,
    answer TEXT NOT NULL,
    images TEXT [],
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS faq_marks (
    question_id INT REFERENCES faq_questions(id) ON UPDATE CASCADE ON DELETE CASCADE,
    user_id UUID REFERENCES users(id) ON UPDATE CASCADE ON DELETE SET NULL,
    anonymous_user_id UUID REFERENCES faq_anonymous_users(id) ON UPDATE CASCADE ON DELETE SET NULL,
    state INT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CHECK (
        (user_id IS NOT NULL AND anonymous_user_id IS NULL)
        OR
        (user_id IS NULL AND anonymous_user_id IS NOT NULL)
    )
);