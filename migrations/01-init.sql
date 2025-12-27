-- migrations/01-init.sql

-- фракции
CREATE TABLE IF NOT EXISTS factions (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL UNIQUE,
    description TEXT,
    faction_influence INTEGER DEFAULT 0,
    is_composition_visible_to_all BOOLEAN DEFAULT false,
    leader_player_id INTEGER
);

-- игроки
CREATE TABLE IF NOT EXISTS players (
    id SERIAL PRIMARY KEY,
    character_name VARCHAR(255) NOT NULL,
    password VARCHAR(255) NOT NULL,
    character_story TEXT,
    role VARCHAR(100) NOT NULL,
    money INTEGER DEFAULT 0 CHECK (money >= 0),
    influence INTEGER DEFAULT 0,
    faction_id INTEGER REFERENCES factions(id) ON DELETE SET NULL,
    can_change_faction BOOLEAN DEFAULT false,
    avatar TEXT -- изображение в формате base64
);

-- информация о других игроках
CREATE TABLE IF NOT EXISTS info_about_other_players (
    id SERIAL PRIMARY KEY,
    player_id INTEGER REFERENCES players(id) on DELETE SET NULL,
    description TEXT
);

-- создаем FK для других игроков
ALTER TABLE factions 
ADD CONSTRAINT fk_leader_player 
FOREIGN KEY (leader_player_id) REFERENCES players(id) ON DELETE SET NULL;

-- предметы
CREATE TABLE IF NOT EXISTS items (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- эффекты
CREATE TABLE IF NOT EXISTS effects (
    id SERIAL PRIMARY KEY,
    description TEXT,
    effect_type VARCHAR(20) NOT NULL, -- 'generate_money', 'generate_influence', 'spawn_item'
    generated_resource VARCHAR(20), -- 'money', 'influence'
    operation VARCHAR(10) DEFAULT 'add', -- 'add', 'mul', 'sub', 'div'
    value INTEGER,
    -- Ð”Ð»Ñ ÑÐ¾Ð·Ð´Ð°Ð½Ð¸Ñ Ð¿Ñ€ÐµÐ´Ð¼ÐµÑ‚Ð¾Ð²
    spawned_item_id INTEGER REFERENCES items(id) ON DELETE CASCADE,
    -- ÐŸÐµÑ€Ð¸Ð¾Ð´ Ð´ÐµÐ¹ÑÑ‚Ð²Ð¸Ñ
    period_seconds INTEGER NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    CHECK (
        (effect_type IN ('generate_money', 'generate_influence') AND generated_resource IS NOT NULL AND value IS NOT NULL AND spawned_item_id IS NULL) OR
        (effect_type = 'spawn_item' AND spawned_item_id IS NOT NULL AND generated_resource IS NULL)
    )
);

-- Ð¡Ð²ÑÐ·ÑŒ Ð²ÐµÑ‰ÐµÐ¹ Ð¸ ÑÑ„Ñ„ÐµÐºÑ‚Ð¾Ð² (Ð¾Ð´Ð½Ð° Ð²ÐµÑ‰ÑŒ Ð¼Ð¾Ð¶ÐµÑ‚ Ð¸Ð¼ÐµÑ‚ÑŒ Ð½ÐµÑÐºÐ¾Ð»ÑŒÐºÐ¾ ÑÑ„Ñ„ÐµÐºÑ‚Ð¾Ð²)
CREATE TABLE IF NOT EXISTS item_effects (
    item_id INTEGER REFERENCES items(id) ON DELETE CASCADE,
    effect_id INTEGER REFERENCES effects(id) ON DELETE CASCADE,
    PRIMARY KEY (item_id, effect_id)
);

-- Ð˜Ð½Ð²ÐµÐ½Ñ‚Ð°Ñ€ÑŒ Ð¸Ð³Ñ€Ð¾ÐºÐ¾Ð²
CREATE TABLE IF NOT EXISTS player_items (
    id SERIAL PRIMARY KEY,
    player_id INTEGER REFERENCES players(id) ON DELETE CASCADE,
    item_id INTEGER REFERENCES items(id) ON DELETE CASCADE,
    acquired_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(player_id, item_id)
);

-- ÐžÑ‚ÑÐ»ÐµÐ¶Ð¸Ð²Ð°Ð½Ð¸Ðµ Ð¿Ð¾ÑÐ»ÐµÐ´Ð½ÐµÐ³Ð¾ Ð²Ñ‹Ð¿Ð¾Ð»Ð½ÐµÐ½Ð¸Ñ ÑÑ„Ñ„ÐµÐºÑ‚Ð¾Ð² Ð²ÐµÑ‰ÐµÐ¹
CREATE TABLE IF NOT EXISTS item_effect_executions (
    id SERIAL PRIMARY KEY,
    player_id INTEGER REFERENCES players(id) ON DELETE CASCADE,
    item_id INTEGER REFERENCES items(id) ON DELETE CASCADE,
    effect_id INTEGER REFERENCES effects(id) ON DELETE CASCADE,
    last_executed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(player_id, item_id, effect_id)
);

-- ============================================
-- Ð¡ÐŸÐžÐ¡ÐžÐ‘ÐÐžÐ¡Ð¢Ð˜
-- ============================================

-- Ð£Ð½Ð¸ÐºÐ°Ð»ÑŒÐ½Ñ‹Ðµ ÑÐ¿Ð¾ÑÐ¾Ð±Ð½Ð¾ÑÑ‚Ð¸
CREATE TABLE IF NOT EXISTS abilities (
    id SERIAL PRIMARY KEY,
    player_id INTEGER REFERENCES players(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    ability_type VARCHAR(50) NOT NULL, -- 'reveal_info', 'add_influence', 'transfer_influence'
    cooldown_minutes INTEGER DEFAULT NULL,
    start_delay_minutes INTEGER DEFAULT NULL, -- Ð·Ð°Ð´ÐµÑ€Ð¶ÐºÐ° Ð¾Ñ‚ Ð½Ð°Ñ‡Ð°Ð»Ð° Ð¸Ð³Ñ€Ñ‹
    required_influence_points INTEGER DEFAULT NULL, -- Ð¼Ð¸Ð½Ð¸Ð¼Ð°Ð»ÑŒÐ½Ð¾Ðµ ÐºÐ¾Ð»Ð¸Ñ‡ÐµÑÑ‚Ð²Ð¾ Ð¾Ñ‡ÐºÐ¾Ð² Ð²Ð»Ð¸ÑÐ½Ð¸Ñ Ð´Ð»Ñ Ñ€Ð°Ð·Ð±Ð»Ð¾ÐºÐ¸Ñ€Ð¾Ð²ÐºÐ¸
    is_unlocked BOOLEAN DEFAULT true, -- Ð±Ñ‹Ð»Ð° Ð»Ð¸ ÑÐ¿Ð¾ÑÐ¾Ð±Ð½Ð¾ÑÑ‚ÑŒ Ñ€Ð°Ð·Ð±Ð»Ð¾ÐºÐ¸Ñ€Ð¾Ð²Ð°Ð½Ð° (Ð¿Ð¾ÑÐ»Ðµ Ñ€Ð°Ð·Ð±Ð»Ð¾ÐºÐ¸Ñ€Ð¾Ð²ÐºÐ¸ Ð¾ÑÑ‚Ð°ÐµÑ‚ÑÑ Ð´Ð¾ÑÑ‚ÑƒÐ¿Ð½Ð¾Ð¹ Ð²ÑÐµÐ³Ð´Ð°)
    -- Ð”Ð»Ñ ÑÐ¿Ð¾ÑÐ¾Ð±Ð½Ð¾ÑÑ‚Ð¸ Ð½Ð°Ñ‡Ð¸ÑÐ»ÐµÐ½Ð¸Ñ Ð²Ð»Ð¸ÑÐ½Ð¸Ñ Ð´Ñ€ÑƒÐ³Ð¾Ð¼Ñƒ Ð¸Ð³Ñ€Ð¾ÐºÑƒ (add_influence)
    influence_points_to_add INTEGER,
    -- Ð”Ð»Ñ ÑÐ¿Ð¾ÑÐ¾Ð±Ð½Ð¾ÑÑ‚Ð¸ ÑÐ½ÑÑ‚Ð¸Ñ Ð²Ð»Ð¸ÑÐ½Ð¸Ñ Ñƒ Ð´Ñ€ÑƒÐ³Ð¾Ð³Ð¾ Ð¸Ð³Ñ€Ð¾ÐºÐ° Ð¸ Ð½Ð°Ñ‡Ð¸ÑÐ»ÐµÐ½Ð¸Ñ ÑÐµÐ±Ðµ (transfer_influence)
    influence_points_to_remove INTEGER, -- ÑÐºÐ¾Ð»ÑŒÐºÐ¾ ÑÐ½ÑÑ‚ÑŒ Ñƒ Ñ†ÐµÐ»ÐµÐ²Ð¾Ð³Ð¾ Ð¸Ð³Ñ€Ð¾ÐºÐ°
    influence_points_to_self INTEGER, -- ÑÐºÐ¾Ð»ÑŒÐºÐ¾ Ð½Ð°Ñ‡Ð¸ÑÐ»Ð¸Ñ‚ÑŒ ÑÐµÐ±Ðµ
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    CHECK (
        (ability_type = 'reveal_info' AND 
         influence_points_to_add IS NULL AND 
         influence_points_to_remove IS NULL AND 
         influence_points_to_self IS NULL) OR
        (ability_type = 'add_influence' AND 
         influence_points_to_add IS NOT NULL AND 
         influence_points_to_remove IS NULL AND 
         influence_points_to_self IS NULL) OR
        (ability_type = 'transfer_influence' AND 
         influence_points_to_add IS NULL AND 
         influence_points_to_remove IS NOT NULL AND 
         influence_points_to_self IS NOT NULL)
    )
);

-- Ð˜ÑÑ‚Ð¾Ñ€Ð¸Ñ Ð¸ÑÐ¿Ð¾Ð»ÑŒÐ·Ð¾Ð²Ð°Ð½Ð¸Ñ ÑÐ¿Ð¾ÑÐ¾Ð±Ð½Ð¾ÑÑ‚ÐµÐ¹ (Ð´Ð»Ñ Ð¾Ñ‚ÑÐ»ÐµÐ¶Ð¸Ð²Ð°Ð½Ð¸Ñ cooldown)
CREATE TABLE IF NOT EXISTS ability_usage (
    id SERIAL PRIMARY KEY,
    player_id INTEGER REFERENCES players(id) ON DELETE CASCADE,
    ability_id INTEGER REFERENCES abilities(id) ON DELETE CASCADE,
    target_player_id INTEGER REFERENCES players(id) ON DELETE SET NULL, -- Ð´Ð»Ñ ÑÐ¿Ð¾ÑÐ¾Ð±Ð½Ð¾ÑÑ‚ÐµÐ¹, Ð½Ð°Ð¿Ñ€Ð°Ð²Ð»ÐµÐ½Ð½Ñ‹Ñ… Ð½Ð° Ð´Ñ€ÑƒÐ³Ð¸Ñ… Ð¸Ð³Ñ€Ð¾ÐºÐ¾Ð²
    info_category VARCHAR(20), -- 'faction', 'goal', 'item' (Ð´Ð»Ñ reveal_info)
    used_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Ð˜ÑÑ‚Ð¾Ñ€Ð¸Ñ Ñ€Ð°ÑÐºÑ€Ñ‹Ñ‚Ð¾Ð¹ Ð¸Ð½Ñ„Ð¾Ñ€Ð¼Ð°Ñ†Ð¸Ð¸
CREATE TABLE IF NOT EXISTS revealed_info (
    id SERIAL PRIMARY KEY,
    revealer_player_id INTEGER REFERENCES players(id) ON DELETE CASCADE,
    target_player_id INTEGER REFERENCES players(id) ON DELETE CASCADE,
    info_type VARCHAR(20) NOT NULL, -- 'faction', 'goal', 'item'
    revealed_data JSONB, -- JSON Ñ Ñ€Ð°ÑÐºÑ€Ñ‹Ñ‚Ð¾Ð¹ Ð¸Ð½Ñ„Ð¾Ñ€Ð¼Ð°Ñ†Ð¸ÐµÐ¹
    revealed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    ability_usage_id INTEGER REFERENCES ability_usage(id) ON DELETE SET NULL
);

-- ============================================
-- Ð¦Ð•Ð›Ð˜
-- ============================================

-- Ð¦ÐµÐ»Ð¸ (Ð»Ð¸Ñ‡Ð½Ñ‹Ðµ Ð¸ Ñ„Ñ€Ð°ÐºÑ†Ð¸Ð¾Ð½Ð½Ñ‹Ðµ)
CREATE TABLE IF NOT EXISTS goals (
    id SERIAL PRIMARY KEY,
    title VARCHAR(255) NOT NULL,
    description TEXT,
    goal_type VARCHAR(20) NOT NULL, -- 'personal', 'faction'
    influence_points_reward INTEGER DEFAULT 0,
    -- Ð”Ð»Ñ Ð»Ð¸Ñ‡Ð½Ñ‹Ñ… Ñ†ÐµÐ»ÐµÐ¹
    player_id INTEGER REFERENCES players(id) ON DELETE CASCADE,
    -- Ð”Ð»Ñ Ñ„Ñ€Ð°ÐºÑ†Ð¸Ð¾Ð½Ð½Ñ‹Ñ… Ñ†ÐµÐ»ÐµÐ¹
    faction_id INTEGER REFERENCES factions(id) ON DELETE CASCADE,
    is_completed BOOLEAN DEFAULT false,
    completed_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    CHECK (
        (goal_type = 'personal' AND player_id IS NOT NULL AND faction_id IS NULL) OR
        (goal_type = 'faction' AND faction_id IS NOT NULL AND player_id IS NULL)
    )
);


-- Ð—Ð°Ð²Ð¸ÑÐ¸Ð¼Ð¾ÑÑ‚Ð¸ Ñ†ÐµÐ»ÐµÐ¹ Ð´Ñ€ÑƒÐ³ Ð¾Ñ‚ Ð´Ñ€ÑƒÐ³Ð° (ÑÐºÑ€Ñ‹Ñ‚Ñ‹Ðµ Ñ†ÐµÐ»Ð¸)
-- ÐžÐ‘ÐÐžÐ’Ð›Ð•ÐÐž: Ð¢ÐµÐ¿ÐµÑ€ÑŒ Ð¿Ð¾Ð´Ð´ÐµÑ€Ð¶Ð¸Ð²Ð°ÐµÑ‚ Ð·Ð°Ð²Ð¸ÑÐ¸Ð¼Ð¾ÑÑ‚ÑŒ Ð¾Ñ‚ Ð²Ð»Ð¸ÑÐ½Ð¸Ñ Ð´Ñ€ÑƒÐ³Ð¸Ñ… Ð¸Ð³Ñ€Ð¾ÐºÐ¾Ð²
CREATE TABLE IF NOT EXISTS goal_dependencies (
    id SERIAL PRIMARY KEY,
    goal_id INTEGER REFERENCES goals(id) ON DELETE CASCADE, -- ÑÑ‚Ð° Ñ†ÐµÐ»ÑŒ Ð·Ð°Ð²Ð¸ÑÐ¸Ñ‚ Ð¾Ñ‚...
    
    -- Ð¢Ð¸Ð¿ Ð·Ð°Ð²Ð¸ÑÐ¸Ð¼Ð¾ÑÑ‚Ð¸
    dependency_type VARCHAR(30) NOT NULL, -- 'goal_completion' Ð¸Ð»Ð¸ 'influence_threshold'
    
    -- Ð”Ð»Ñ Ð·Ð°Ð²Ð¸ÑÐ¸Ð¼Ð¾ÑÑ‚Ð¸ Ð¾Ñ‚ Ð²Ñ‹Ð¿Ð¾Ð»Ð½ÐµÐ½Ð¸Ñ Ð´Ñ€ÑƒÐ³Ð¾Ð¹ Ñ†ÐµÐ»Ð¸
    required_goal_id INTEGER REFERENCES goals(id) ON DELETE CASCADE,
    
    -- Ð”Ð»Ñ Ð·Ð°Ð²Ð¸ÑÐ¸Ð¼Ð¾ÑÑ‚Ð¸ Ð¾Ñ‚ Ð¾Ñ‡ÐºÐ¾Ð² Ð²Ð»Ð¸ÑÐ½Ð¸Ñ Ð´Ñ€ÑƒÐ³Ð¾Ð³Ð¾ Ð¸Ð³Ñ€Ð¾ÐºÐ°
    influence_player_id INTEGER REFERENCES players(id) ON DELETE CASCADE,
    required_influence_points INTEGER,
    
    -- Ð’Ð¸Ð´Ð¸Ð¼Ð¾ÑÑ‚ÑŒ Ð´Ð¾ Ð²Ñ‹Ð¿Ð¾Ð»Ð½ÐµÐ½Ð¸Ñ ÑƒÑÐ»Ð¾Ð²Ð¸Ñ
    is_visible_before_completion BOOLEAN DEFAULT false, -- false = Ð¿Ð¾Ð»Ð½Ð¾ÑÑ‚ÑŒÑŽ ÑÐºÑ€Ñ‹Ñ‚Ð°; true = Ð²Ð¸Ð´Ð½Ð°, Ð½Ð¾ Ð·Ð°Ð±Ð»Ð¾ÐºÐ¸Ñ€Ð¾Ð²Ð°Ð½Ð°
    
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    -- ÐŸÑ€Ð¾Ð²ÐµÑ€ÐºÐ¸ Ñ†ÐµÐ»Ð¾ÑÑ‚Ð½Ð¾ÑÑ‚Ð¸ Ð´Ð°Ð½Ð½Ñ‹Ñ…
    CHECK (
        -- Ð”Ð»Ñ Ñ‚Ð¸Ð¿Ð° 'goal_completion' Ð´Ð¾Ð»Ð¶ÐµÐ½ Ð±Ñ‹Ñ‚ÑŒ ÑƒÐºÐ°Ð·Ð°Ð½ required_goal_id
        (dependency_type = 'goal_completion' AND 
         required_goal_id IS NOT NULL AND 
         influence_player_id IS NULL AND 
         required_influence_points IS NULL) OR
        -- Ð”Ð»Ñ Ñ‚Ð¸Ð¿Ð° 'influence_threshold' Ð´Ð¾Ð»Ð¶Ð½Ñ‹ Ð±Ñ‹Ñ‚ÑŒ ÑƒÐºÐ°Ð·Ð°Ð½Ñ‹ influence_player_id Ð¸ required_influence_points
        (dependency_type = 'influence_threshold' AND 
         required_goal_id IS NULL AND 
         influence_player_id IS NOT NULL AND 
         required_influence_points IS NOT NULL AND
         required_influence_points > 0)
    ),
    
    -- Ð¦ÐµÐ»ÑŒ Ð½Ðµ Ð¼Ð¾Ð¶ÐµÑ‚ Ð·Ð°Ð²Ð¸ÑÐµÑ‚ÑŒ Ð¾Ñ‚ ÑÐ°Ð¼Ð¾Ð¹ ÑÐµÐ±Ñ
    CHECK (goal_id != required_goal_id),
    
    -- Ð£Ð½Ð¸ÐºÐ°Ð»ÑŒÐ½Ð¾ÑÑ‚ÑŒ Ð·Ð°Ð²Ð¸ÑÐ¸Ð¼Ð¾ÑÑ‚ÐµÐ¹
    UNIQUE(goal_id, dependency_type, required_goal_id),
    UNIQUE(goal_id, dependency_type, influence_player_id)
);

-- Ð˜ÑÑ‚Ð¾Ñ€Ð¸Ñ Ñ€Ð°Ð·Ð±Ð»Ð¾ÐºÐ¸Ñ€Ð¾Ð²Ð¾Ðº Ð·Ð°Ð²Ð¸ÑÐ¸Ð¼Ð¾ÑÑ‚ÐµÐ¹ Ñ†ÐµÐ»ÐµÐ¹
-- ÐšÐ¾Ð³Ð´Ð° Ð·Ð°Ð²Ð¸ÑÐ¸Ð¼Ð¾ÑÑ‚ÑŒ Ñ€Ð°Ð·Ð±Ð»Ð¾ÐºÐ¸Ñ€ÑƒÐµÑ‚ÑÑ (Ñ†ÐµÐ»ÑŒ Ð²Ñ‹Ð¿Ð¾Ð»Ð½ÐµÐ½Ð° Ð¸Ð»Ð¸ Ð¿Ð¾Ñ€Ð¾Ð³ Ð²Ð»Ð¸ÑÐ½Ð¸Ñ Ð´Ð¾ÑÑ‚Ð¸Ð³Ð½ÑƒÑ‚),
-- Ð·Ð°Ð¿Ð¸ÑÑŒ Ð´Ð¾Ð±Ð°Ð²Ð»ÑÐµÑ‚ÑÑ Ð² ÑÑ‚Ñƒ Ñ‚Ð°Ð±Ð»Ð¸Ñ†Ñƒ Ð¸ Ð¾ÑÑ‚Ð°Ñ‘Ñ‚ÑÑ Ñ‚Ð°Ð¼ Ð½Ð°Ð²ÑÐµÐ³Ð´Ð°
-- Ð­Ñ‚Ð¾ Ð³Ð°Ñ€Ð°Ð½Ñ‚Ð¸Ñ€ÑƒÐµÑ‚, Ñ‡Ñ‚Ð¾ Ñ†ÐµÐ»ÑŒ Ð¾ÑÑ‚Ð°Ñ‘Ñ‚ÑÑ Ð´Ð¾ÑÑ‚ÑƒÐ¿Ð½Ð¾Ð¹ Ð´Ð°Ð¶Ðµ ÐµÑÐ»Ð¸ ÑƒÑÐ»Ð¾Ð²Ð¸Ðµ Ð¿ÐµÑ€ÐµÑÑ‚Ð°Ð»Ð¾ Ð²Ñ‹Ð¿Ð¾Ð»Ð½ÑÑ‚ÑŒÑÑ
CREATE TABLE IF NOT EXISTS goal_dependency_unlocks (
    id SERIAL PRIMARY KEY,
    goal_id INTEGER REFERENCES goals(id) ON DELETE CASCADE,
    dependency_id INTEGER REFERENCES goal_dependencies(id) ON DELETE CASCADE,
    player_id INTEGER REFERENCES players(id) ON DELETE CASCADE, -- Ð²Ð»Ð°Ð´ÐµÐ»ÐµÑ† Ñ†ÐµÐ»Ð¸
    unlocked_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    -- ÐžÐ´Ð½Ð° Ð·Ð°Ð²Ð¸ÑÐ¸Ð¼Ð¾ÑÑ‚ÑŒ Ð¼Ð¾Ð¶ÐµÑ‚ Ð±Ñ‹Ñ‚ÑŒ Ñ€Ð°Ð·Ð±Ð»Ð¾ÐºÐ¸Ñ€Ð¾Ð²Ð°Ð½Ð° Ñ‚Ð¾Ð»ÑŒÐºÐ¾ Ð¾Ð´Ð¸Ð½ Ñ€Ð°Ð· Ð´Ð»Ñ Ð¾Ð´Ð½Ð¾Ð¹ Ñ†ÐµÐ»Ð¸
    UNIQUE(goal_id, dependency_id)
);

-- Ð˜ÑÑ‚Ð¾Ñ€Ð¸Ñ Ð²Ñ‹Ð¿Ð¾Ð»Ð½ÐµÐ½Ð¸Ñ Ñ†ÐµÐ»ÐµÐ¹ (Ð´Ð»Ñ Ð¾Ñ‚ÑÐ»ÐµÐ¶Ð¸Ð²Ð°Ð½Ð¸Ñ Ð½Ð°Ñ‡Ð¸ÑÐ»ÐµÐ½Ð¸Ñ/ÑÐ½ÑÑ‚Ð¸Ñ Ð¾Ñ‡ÐºÐ¾Ð² Ð²Ð»Ð¸ÑÐ½Ð¸Ñ)
CREATE TABLE IF NOT EXISTS goal_completion_history (
    id SERIAL PRIMARY KEY,
    goal_id INTEGER REFERENCES goals(id) ON DELETE CASCADE,
    player_id INTEGER REFERENCES players(id) ON DELETE CASCADE, -- ÐºÑ‚Ð¾ Ð¾Ñ‚Ð¼ÐµÑ‚Ð¸Ð» Ñ†ÐµÐ»ÑŒ
    action VARCHAR(20) NOT NULL, -- 'completed', 'uncompleted'
    influence_change INTEGER NOT NULL, -- Ð¸Ð·Ð¼ÐµÐ½ÐµÐ½Ð¸Ðµ Ð²Ð»Ð¸ÑÐ½Ð¸Ñ
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- ============================================
-- Ð—ÐÐ”ÐÐ§Ð˜ Ð˜ Ð“ÐžÐÐšÐ Ð¦Ð•Ð›Ð•Ð™
-- ============================================

-- Ð—Ð°Ð´Ð°Ñ‡Ð¸ Ð¸Ð³Ñ€Ð¾ÐºÐ¾Ð² (Ð¾Ñ‚Ð»Ð¸Ñ‡Ð°ÑŽÑ‚ÑÑ Ð¾Ñ‚ Ñ†ÐµÐ»ÐµÐ¹)
CREATE TABLE IF NOT EXISTS tasks (
    id SERIAL PRIMARY KEY,
    player_id INTEGER REFERENCES players(id) ON DELETE CASCADE,
    title VARCHAR(255) NOT NULL,
    description TEXT,
    is_completed BOOLEAN DEFAULT false,
    completed_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Ð˜ÑÑ‚Ð¾Ñ€Ð¸Ñ Ð²Ñ‹Ð¿Ð¾Ð»Ð½ÐµÐ½Ð¸Ñ Ð·Ð°Ð´Ð°Ñ‡
CREATE TABLE IF NOT EXISTS task_completion_history (
    id SERIAL PRIMARY KEY,
    task_id INTEGER REFERENCES tasks(id) ON DELETE CASCADE,
    player_id INTEGER REFERENCES players(id) ON DELETE CASCADE,
    action VARCHAR(20) NOT NULL, -- 'completed', 'uncompleted'
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Ð£ÑÐ»Ð¾Ð²Ð¸Ñ Ð´Ð»Ñ Ð·Ð°Ð¿ÑƒÑÐºÐ° Ð³Ð¾Ð½ÐºÐ¸ Ñ†ÐµÐ»ÐµÐ¹
CREATE TABLE IF NOT EXISTS goal_race_triggers (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    required_tasks_count INTEGER NOT NULL CHECK (required_tasks_count > 0),
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Ð˜Ð³Ñ€Ð¾ÐºÐ¸, ÑƒÑ‡Ð°ÑÑ‚Ð²ÑƒÑŽÑ‰Ð¸Ðµ Ð² Ð³Ð¾Ð½ÐºÐµ Ð¿Ñ€Ð¸ ÑÑ€Ð°Ð±Ð°Ñ‚Ñ‹Ð²Ð°Ð½Ð¸Ð¸ Ñ‚Ñ€Ð¸Ð³Ð³ÐµÑ€Ð°
CREATE TABLE IF NOT EXISTS goal_race_trigger_participants (
    id SERIAL PRIMARY KEY,
    trigger_id INTEGER REFERENCES goal_race_triggers(id) ON DELETE CASCADE,
    player_id INTEGER REFERENCES players(id) ON DELETE CASCADE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(trigger_id, player_id)
);

-- Ð Ð°ÑƒÐ½Ð´Ñ‹ Ð³Ð¾Ð½ÐºÐ¸ Ñ†ÐµÐ»ÐµÐ¹
CREATE TABLE IF NOT EXISTS goal_race_rounds (
    id SERIAL PRIMARY KEY,
    trigger_id INTEGER REFERENCES goal_race_triggers(id) ON DELETE SET NULL,
    round_number INTEGER NOT NULL DEFAULT 1, -- Ð½Ð¾Ð¼ÐµÑ€ Ñ€Ð°ÑƒÐ½Ð´Ð° Ð² Ñ€Ð°Ð¼ÐºÐ°Ñ… Ð¾Ð´Ð½Ð¾Ð¹ Ð³Ð¾Ð½ÐºÐ¸
    status VARCHAR(20) DEFAULT 'pending', -- 'pending', 'active', 'completed', 'cancelled'
    started_at TIMESTAMP,
    completed_at TIMESTAMP,
    winner_player_id INTEGER REFERENCES players(id) ON DELETE SET NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Ð£Ñ‡Ð°ÑÑ‚Ð½Ð¸ÐºÐ¸ ÐºÐ¾Ð½ÐºÑ€ÐµÑ‚Ð½Ð¾Ð³Ð¾ Ñ€Ð°ÑƒÐ½Ð´Ð°
CREATE TABLE IF NOT EXISTS goal_race_round_participants (
    id SERIAL PRIMARY KEY,
    round_id INTEGER REFERENCES goal_race_rounds(id) ON DELETE CASCADE,
    player_id INTEGER REFERENCES players(id) ON DELETE CASCADE,
    joined_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(round_id, player_id)
);

-- ÐŸÑ€ÐµÐ´Ð¾Ð¿Ñ€ÐµÐ´ÐµÐ»ÐµÐ½Ð½Ñ‹Ðµ Ñ†ÐµÐ»Ð¸ Ð´Ð»Ñ Ñ€Ð°ÑƒÐ½Ð´Ð¾Ð² Ð³Ð¾Ð½ÐºÐ¸
-- ÐÐ´Ð¼Ð¸Ð½ ÑÐ¾Ð·Ð´Ð°ÐµÑ‚ ÑÑ‚Ð¸ Ñ†ÐµÐ»Ð¸ Ð—ÐÐ ÐÐÐ•Ð•, Ð´Ð¾ Ð·Ð°Ð¿ÑƒÑÐºÐ° Ð³Ð¾Ð½ÐºÐ¸
CREATE TABLE IF NOT EXISTS goal_race_predefined_goals (
    id SERIAL PRIMARY KEY,
    trigger_id INTEGER REFERENCES goal_race_triggers(id) ON DELETE CASCADE,
    round_number INTEGER NOT NULL, -- Ð´Ð»Ñ ÐºÐ°ÐºÐ¾Ð³Ð¾ Ñ€Ð°ÑƒÐ½Ð´Ð° ÑÑ‚Ð° Ñ†ÐµÐ»ÑŒ
    player_id INTEGER REFERENCES players(id) ON DELETE CASCADE, -- ÐºÐ¾Ð¼Ñƒ Ð½Ð°Ð·Ð½Ð°Ñ‡ÐµÐ½Ð° Ñ†ÐµÐ»ÑŒ
    title VARCHAR(255) NOT NULL,
    description TEXT,
    influence_points_reward INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(trigger_id, round_number, player_id, title) -- Ð¾Ð´Ð¸Ð½ Ð¸Ð³Ñ€Ð¾Ðº Ð½Ðµ Ð¼Ð¾Ð¶ÐµÑ‚ Ð¿Ð¾Ð»ÑƒÑ‡Ð¸Ñ‚ÑŒ Ð¾Ð´Ð¸Ð½Ð°ÐºÐ¾Ð²ÑƒÑŽ Ñ†ÐµÐ»ÑŒ Ð² Ñ€Ð°ÑƒÐ½Ð´Ðµ Ð´Ð²Ð°Ð¶Ð´Ñ‹
);

-- Ð¡Ð²ÑÐ·ÑŒ Ñ†ÐµÐ»ÐµÐ¹ Ñ Ñ€Ð°ÑƒÐ½Ð´Ð°Ð¼Ð¸ Ð³Ð¾Ð½ÐºÐ¸ (ÑÐ¾Ð·Ð´Ð°ÐµÑ‚ÑÑ Ð¿Ñ€Ð¸ Ð°ÐºÑ‚Ð¸Ð²Ð°Ñ†Ð¸Ð¸ Ñ€Ð°ÑƒÐ½Ð´Ð°)
CREATE TABLE IF NOT EXISTS goal_race_round_goals (
    id SERIAL PRIMARY KEY,
    round_id INTEGER REFERENCES goal_race_rounds(id) ON DELETE CASCADE,
    goal_id INTEGER REFERENCES goals(id) ON DELETE CASCADE,
    assigned_player_id INTEGER REFERENCES players(id) ON DELETE CASCADE,
    is_accessible BOOLEAN DEFAULT true, -- false ÐºÐ¾Ð³Ð´Ð° Ñ€Ð°ÑƒÐ½Ð´ Ð·Ð°Ð²ÐµÑ€ÑˆÐ°ÐµÑ‚ÑÑ
    assigned_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    became_inaccessible_at TIMESTAMP,
    UNIQUE(round_id, goal_id),
    UNIQUE(round_id, assigned_player_id, goal_id)
);

-- ============================================
-- Ð”ÐžÐ“ÐžÐ’ÐžÐ Ð«
-- ============================================

-- Ð”Ð¾Ð³Ð¾Ð²Ð¾Ñ€Ñ‹ Ð¼ÐµÐ¶Ð´Ñƒ Ð¸Ð³Ñ€Ð¾ÐºÐ°Ð¼Ð¸
CREATE TABLE IF NOT EXISTS contracts (
    id SERIAL PRIMARY KEY,
    contract_type VARCHAR(20) NOT NULL, -- 'type1', 'type2'
    customer_player_id INTEGER REFERENCES players(id) ON DELETE CASCADE,
    executor_player_id INTEGER REFERENCES players(id) ON DELETE CASCADE,
    customer_faction_id INTEGER REFERENCES factions(id) ON DELETE SET NULL, -- Ñ„Ñ€Ð°ÐºÑ†Ð¸Ñ Ð·Ð°ÐºÐ°Ð·Ñ‡Ð¸ÐºÐ° Ð½Ð° Ð¼Ð¾Ð¼ÐµÐ½Ñ‚ Ð¿Ð¾Ð´Ð¿Ð¸ÑÐ°Ð½Ð¸Ñ
    status VARCHAR(20) DEFAULT 'pending', -- 'pending', 'signed', 'completed', 'terminated'
    duration_seconds INTEGER NOT NULL,
    money_reward_customer INTEGER DEFAULT 0, -- Ð´ÐµÐ½ÑŒÐ³Ð¸ Ð´Ð»Ñ Ð·Ð°ÐºÐ°Ð·Ñ‡Ð¸ÐºÐ°
    money_reward_executor INTEGER DEFAULT 0, -- Ð´ÐµÐ½ÑŒÐ³Ð¸ Ð´Ð»Ñ Ð¸ÑÐ¿Ð¾Ð»Ð½Ð¸Ñ‚ÐµÐ»Ñ
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    signed_at TIMESTAMP,
    expires_at TIMESTAMP,
    completed_at TIMESTAMP,
    terminated_at TIMESTAMP,
    CHECK (customer_player_id != executor_player_id)
);

-- Настройки времени для договоров
CREATE TABLE IF NOT EXISTS contract_duration_settings (
    id SERIAL PRIMARY KEY,
    type VARCHAR(5), -- 'type1', 'type2'
    duration_minutes INTEGER
);


-- ÐÐ°ÑÑ‚Ñ€Ð¾Ð¹ÐºÐ¸ Ð½Ð°Ð³Ñ€Ð°Ð´Ñ‹ Ð´Ð»Ñ ÐºÐ¾Ð½Ñ‚Ñ€Ð°ÐºÑ‚Ð° Ñ‚Ð¸Ð¿Ð° 1 (Ð²ÐµÑ‰Ð¸ Ð¿Ð¾ Ñ„Ñ€Ð°ÐºÑ†Ð¸ÑÐ¼)
CREATE TABLE IF NOT EXISTS contract_type1_settings (
    id SERIAL PRIMARY KEY,
    faction_id INTEGER REFERENCES factions(id) ON DELETE CASCADE,
    customer_item_reward_id INTEGER REFERENCES items(id) ON DELETE SET NULL,
    UNIQUE(faction_id)
);

-- ÐÐ°ÑÑ‚Ñ€Ð¾Ð¹ÐºÐ¸ Ð½Ð°Ð³Ñ€Ð°Ð´ Ð´Ð»Ñ Ð´Ð¾Ð³Ð¾Ð²Ð¾Ñ€Ð¾Ð² Type 1
CREATE TABLE IF NOT EXISTS contract_type1_reward_settings (
    id SERIAL PRIMARY KEY,
    money_reward_customer INTEGER DEFAULT 0 CHECK (money_reward_customer >= 0),
    money_reward_executor INTEGER DEFAULT 0 CHECK (money_reward_executor >= 0),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- ÐÐ°ÑÑ‚Ñ€Ð¾Ð¹ÐºÐ¸ Ð½Ð°Ð³Ñ€Ð°Ð´ Ð´Ð»Ñ Ð´Ð¾Ð³Ð¾Ð²Ð¾Ñ€Ð¾Ð² Type 2
CREATE TABLE IF NOT EXISTS contract_type2_reward_settings (
    id SERIAL PRIMARY KEY,
    money_reward_executor INTEGER DEFAULT 0 CHECK (money_reward_executor >= 0),
    -- money_reward_customer Ð²ÑÐµÐ³Ð´Ð° 0 Ð´Ð»Ñ type2
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- ÐÐ°ÑÑ‚Ñ€Ð¾Ð¹ÐºÐ¸ ÑˆÑ‚Ñ€Ð°Ñ„Ð¾Ð² Ð·Ð° Ð½Ð°Ñ€ÑƒÑˆÐµÐ½Ð¸Ðµ Ð´Ð¾Ð³Ð¾Ð²Ð¾Ñ€Ð°
CREATE TABLE IF NOT EXISTS contract_penalty_settings (
    id SERIAL PRIMARY KEY,
    money_penalty INTEGER DEFAULT 0,
    influence_penalty INTEGER DEFAULT 0
);

-- Ð˜ÑÑ‚Ð¾Ñ€Ð¸Ñ ÑˆÑ‚Ñ€Ð°Ñ„Ð¾Ð² Ð¿Ð¾ Ð´Ð¾Ð³Ð¾Ð²Ð¾Ñ€Ð°Ð¼
CREATE TABLE IF NOT EXISTS contract_penalties (
    id SERIAL PRIMARY KEY,
    player_id INTEGER REFERENCES players(id) ON DELETE CASCADE,
    contract_id INTEGER REFERENCES contracts(id) ON DELETE SET NULL,
    violation_type VARCHAR(50) NOT NULL, -- 'faction_conflict'
    money_penalty INTEGER DEFAULT 0,
    influence_penalty INTEGER DEFAULT 0,
    applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- ============================================
-- Ð”ÐžÐ›Ð“ÐžÐ’Ð«Ð• Ð ÐÐ¡ÐŸÐ˜Ð¡ÐšÐ˜
-- ============================================

-- Ð”Ð¾Ð»Ð³Ð¾Ð²Ñ‹Ðµ Ñ€Ð°ÑÐ¿Ð¸ÑÐºÐ¸
CREATE TABLE IF NOT EXISTS debt_receipts (
    id SERIAL PRIMARY KEY,    
    lender_player_id INTEGER REFERENCES players(id) ON DELETE CASCADE, -- ÐºÑ€ÐµÐ´Ð¸Ñ‚Ð¾Ñ€
    borrower_player_id INTEGER REFERENCES players(id) ON DELETE CASCADE, -- Ð·Ð°ÐµÐ¼Ñ‰Ð¸Ðº
    loan_amount INTEGER NOT NULL CHECK (loan_amount > 0),
    return_amount INTEGER NOT NULL CHECK (return_amount > 0),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    return_deadline TIMESTAMP NOT NULL,
    is_returned BOOLEAN DEFAULT false,
    returned_at TIMESTAMP,
    penalty_applied BOOLEAN DEFAULT false,
    penalty_applied_at TIMESTAMP,
    CHECK (lender_player_id != borrower_player_id)
);

-- ÐÐ°ÑÑ‚Ñ€Ð¾Ð¹ÐºÐ¸ ÑˆÑ‚Ñ€Ð°Ñ„Ð¾Ð² Ð´Ð»Ñ Ð´Ð¾Ð»Ð³Ð¾Ð²Ñ‹Ñ… Ñ€Ð°ÑÐ¿Ð¸ÑÐ¾Ðº
CREATE TABLE IF NOT EXISTS debt_penalty_settings (
    id SERIAL PRIMARY KEY,
    penalty_influence_points INTEGER DEFAULT 0
);

-- ============================================
-- Ð¢Ð ÐÐÐ—ÐÐšÐ¦Ð˜Ð˜
-- ============================================

-- Ð”ÐµÐ½ÐµÐ¶Ð½Ñ‹Ðµ Ñ‚Ñ€Ð°Ð½Ð·Ð°ÐºÑ†Ð¸Ð¸ (Ð´Ð»Ñ Ð¾Ñ‚ÑÐ»ÐµÐ¶Ð¸Ð²Ð°Ð½Ð¸Ñ Ð°Ð´Ð¼Ð¸Ð½Ð°Ð¼Ð¸)
CREATE TABLE IF NOT EXISTS money_transactions (
    id SERIAL PRIMARY KEY,
    from_player_id INTEGER REFERENCES players(id) ON DELETE SET NULL,
    to_player_id INTEGER REFERENCES players(id) ON DELETE SET NULL,
    amount INTEGER NOT NULL,
    transaction_type VARCHAR(50) NOT NULL, -- 'transfer', 'contract', 'debt', 'penalty', 'item_effect'
    reference_id INTEGER, -- ID ÑÐ²ÑÐ·Ð°Ð½Ð½Ð¾Ð³Ð¾ Ð´Ð¾Ð³Ð¾Ð²Ð¾Ñ€Ð°, Ð´Ð¾Ð»Ð³Ð° Ð¸ Ñ‚.Ð´.
    reference_type VARCHAR(50), -- 'contract', 'debt_receipt', 'effect'
    description TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Ð¢Ñ€Ð°Ð½Ð·Ð°ÐºÑ†Ð¸Ð¸ Ð¿Ñ€ÐµÐ´Ð¼ÐµÑ‚Ð¾Ð²
CREATE TABLE IF NOT EXISTS item_transactions (
    id SERIAL PRIMARY KEY,
    from_player_id INTEGER REFERENCES players(id) ON DELETE SET NULL,
    to_player_id INTEGER REFERENCES players(id) ON DELETE SET NULL,
    item_id INTEGER REFERENCES items(id) ON DELETE SET NULL,
    transaction_type VARCHAR(50) NOT NULL, -- 'transfer', 'contract', 'spawned'
    reference_id INTEGER,
    reference_type VARCHAR(50),
    description TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Ð¢Ñ€Ð°Ð½Ð·Ð°ÐºÑ†Ð¸Ð¸ Ð²Ð»Ð¸ÑÐ½Ð¸Ñ
CREATE TABLE IF NOT EXISTS influence_transactions (
    id SERIAL PRIMARY KEY,
    player_id INTEGER REFERENCES players(id) ON DELETE SET NULL,
    amount INTEGER NOT NULL,
    transaction_type VARCHAR(50) NOT NULL, -- 'goal', 'penalty', 'ability', 'item_effect'
    reference_id INTEGER,
    reference_type VARCHAR(50),
    description TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- ============================================
-- Ð˜Ð“Ð ÐžÐ’Ð«Ð• ÐÐÐ¡Ð¢Ð ÐžÐ™ÐšÐ˜
-- ============================================

-- ÐžÐ±Ñ‰Ð¸Ðµ Ð½Ð°ÑÑ‚Ñ€Ð¾Ð¹ÐºÐ¸ Ð¸Ð³Ñ€Ñ‹
CREATE TABLE IF NOT EXISTS game_settings (
    id SERIAL PRIMARY KEY,
    setting_key VARCHAR(100) NOT NULL UNIQUE,
    setting_value TEXT,
    description TEXT,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Ð’Ñ€ÐµÐ¼ÐµÐ½Ð½Ñ‹Ðµ Ð¼ÐµÑ‚ÐºÐ¸ Ð¸Ð³Ñ€Ñ‹
CREATE TABLE IF NOT EXISTS game_timeline (
    id SERIAL PRIMARY KEY,
    game_started_at TIMESTAMP,
    game_ended_at TIMESTAMP
);

-- ============================================
-- Ð˜ÐÐ”Ð•ÐšÐ¡Ð« Ð”Ð›Ð¯ ÐŸÐ ÐžÐ˜Ð—Ð’ÐžÐ”Ð˜Ð¢Ð•Ð›Ð¬ÐÐžÐ¡Ð¢Ð˜
-- ============================================

-- Ð˜Ð½Ð´ÐµÐºÑÑ‹ Ð´Ð»Ñ Ñ‡Ð°ÑÑ‚Ñ‹Ñ… Ð·Ð°Ð¿Ñ€Ð¾ÑÐ¾Ð²
CREATE INDEX idx_players_faction ON players(faction_id);
CREATE INDEX idx_goals_player ON goals(player_id);
CREATE INDEX idx_goals_faction ON goals(faction_id);
CREATE INDEX idx_goals_type ON goals(goal_type);
CREATE INDEX idx_contracts_customer ON contracts(customer_player_id);
CREATE INDEX idx_contracts_executor ON contracts(executor_player_id);
CREATE INDEX idx_contracts_status ON contracts(status);
CREATE INDEX idx_debt_receipts_lender ON debt_receipts(lender_player_id);
CREATE INDEX idx_debt_receipts_borrower ON debt_receipts(borrower_player_id);
CREATE INDEX idx_debt_receipts_deadline ON debt_receipts(return_deadline);
CREATE INDEX idx_player_items_player ON player_items(player_id);
CREATE INDEX idx_ability_usage_player ON ability_usage(player_id);
CREATE INDEX idx_ability_usage_ability ON ability_usage(ability_id);
CREATE INDEX idx_item_effect_executions_player ON item_effect_executions(player_id);

-- Ð˜Ð½Ð´ÐµÐºÑÑ‹ Ð´Ð»Ñ Ð·Ð°Ð´Ð°Ñ‡ Ð¸ Ð³Ð¾Ð½ÐºÐ¸ Ñ†ÐµÐ»ÐµÐ¹
CREATE INDEX idx_tasks_player ON tasks(player_id);
CREATE INDEX idx_tasks_completed ON tasks(is_completed);
CREATE INDEX idx_goal_race_rounds_status ON goal_race_rounds(status);
CREATE INDEX idx_goal_race_rounds_trigger ON goal_race_rounds(trigger_id);
CREATE INDEX idx_goal_race_predefined_goals_trigger_round ON goal_race_predefined_goals(trigger_id, round_number);
CREATE INDEX idx_goal_race_predefined_goals_player ON goal_race_predefined_goals(player_id);
CREATE INDEX idx_goal_race_round_goals_round ON goal_race_round_goals(round_id);
CREATE INDEX idx_goal_race_round_goals_player ON goal_race_round_goals(assigned_player_id);
CREATE INDEX idx_goal_race_round_goals_accessible ON goal_race_round_goals(is_accessible);

-- ============================================
-- ÐŸÐ Ð•Ð”Ð¡Ð¢ÐÐ’Ð›Ð•ÐÐ˜Ð¯ (VIEWS)
-- ============================================

-- ÐŸÑ€ÐµÐ´ÑÑ‚Ð°Ð²Ð»ÐµÐ½Ð¸Ðµ Ð´Ð»Ñ Ð¿Ð¾Ð´ÑÑ‡ÐµÑ‚Ð° Ð¾Ð±Ñ‰ÐµÐ³Ð¾ Ð²Ð»Ð¸ÑÐ½Ð¸Ñ Ñ„Ñ€Ð°ÐºÑ†Ð¸Ð¸
CREATE OR REPLACE VIEW faction_total_influence AS
SELECT 
    f.id AS faction_id,
    f.name AS faction_name,
    f.faction_influence AS faction_own_influence,
    COALESCE(SUM(p.influence), 0) AS players_total_influence,
    f.faction_influence + COALESCE(SUM(p.influence), 0) AS total_influence
FROM factions f
LEFT JOIN players p ON p.faction_id = f.id
GROUP BY f.id, f.name, f.faction_influence;


-- ÐŸÑ€ÐµÐ´ÑÑ‚Ð°Ð²Ð»ÐµÐ½Ð¸Ðµ Ð´Ð»Ñ Ð²Ð¸Ð´Ð¸Ð¼Ñ‹Ñ… Ñ†ÐµÐ»ÐµÐ¹ Ð¸Ð³Ñ€Ð¾ÐºÐ°
-- ÐžÐ‘ÐÐžÐ’Ð›Ð•ÐÐž: Ð£Ñ‡Ð¸Ñ‚Ñ‹Ð²Ð°ÐµÑ‚ Ñ€Ð°Ð·Ð±Ð»Ð¾ÐºÐ¸Ñ€Ð¾Ð²ÐºÐ¸ Ð¸Ð· goal_dependency_unlocks
CREATE OR REPLACE VIEW player_visible_goals AS
SELECT 
    g.id,
    g.title,
    g.description,
    g.player_id,
    g.influence_points_reward,
    g.is_completed,
    -- ÐžÐ¿Ñ€ÐµÐ´ÐµÐ»ÑÐµÐ¼ Ð²Ð¸Ð´Ð¸Ð¼Ð¾ÑÑ‚ÑŒ Ñ†ÐµÐ»Ð¸
    CASE 
        -- Ð•ÑÐ»Ð¸ Ð½ÐµÑ‚ Ð·Ð°Ð²Ð¸ÑÐ¸Ð¼Ð¾ÑÑ‚ÐµÐ¹ - Ñ†ÐµÐ»ÑŒ Ð²Ð¸Ð´Ð½Ð°
        WHEN NOT EXISTS (
            SELECT 1 FROM goal_dependencies gd WHERE gd.goal_id = g.id
        ) THEN true
        
        -- Ð•ÑÐ»Ð¸ ÐµÑÑ‚ÑŒ Ð·Ð°Ð²Ð¸ÑÐ¸Ð¼Ð¾ÑÑ‚ÑŒ Ñ is_visible_before_completion = true - Ñ†ÐµÐ»ÑŒ Ð²Ð¸Ð´Ð½Ð° (Ð½Ð¾ Ð¼Ð¾Ð¶ÐµÑ‚ Ð±Ñ‹Ñ‚ÑŒ Ð·Ð°Ð±Ð»Ð¾ÐºÐ¸Ñ€Ð¾Ð²Ð°Ð½Ð°)
        WHEN EXISTS (
            SELECT 1 FROM goal_dependencies gd 
            WHERE gd.goal_id = g.id 
            AND gd.is_visible_before_completion = true
        ) THEN true
        
        -- ÐŸÑ€Ð¾Ð²ÐµÑ€ÑÐµÐ¼, Ð²Ñ‹Ð¿Ð¾Ð»Ð½ÐµÐ½Ñ‹ Ð»Ð¸ Ð’Ð¡Ð• ÑƒÑÐ»Ð¾Ð²Ð¸Ñ Ð·Ð°Ð²Ð¸ÑÐ¸Ð¼Ð¾ÑÑ‚Ð¸ (Ñ ÑƒÑ‡ÐµÑ‚Ð¾Ð¼ unlocks!)
        WHEN NOT EXISTS (
            SELECT 1 FROM goal_dependencies gd
            LEFT JOIN goals rg ON gd.required_goal_id = rg.id
            LEFT JOIN players p ON gd.influence_player_id = p.id
            LEFT JOIN goal_dependency_unlocks gdu ON gd.id = gdu.dependency_id AND gd.goal_id = gdu.goal_id
            WHERE gd.goal_id = g.id 
            AND gdu.id IS NULL  -- Ð—Ð°Ð²Ð¸ÑÐ¸Ð¼Ð¾ÑÑ‚ÑŒ ÐµÑ‰Ñ‘ Ð½Ðµ Ñ€Ð°Ð·Ð±Ð»Ð¾ÐºÐ¸Ñ€Ð¾Ð²Ð°Ð½Ð°
            AND (
                -- Ð—Ð°Ð²Ð¸ÑÐ¸Ð¼Ð¾ÑÑ‚ÑŒ Ð¾Ñ‚ Ñ†ÐµÐ»Ð¸ Ð½Ðµ Ð²Ñ‹Ð¿Ð¾Ð»Ð½ÐµÐ½Ð°
                (gd.dependency_type = 'goal_completion' AND (rg.is_completed = false OR rg.is_completed IS NULL))
                OR
                -- Ð—Ð°Ð²Ð¸ÑÐ¸Ð¼Ð¾ÑÑ‚ÑŒ Ð¾Ñ‚ Ð¾Ñ‡ÐºÐ¾Ð² Ð²Ð»Ð¸ÑÐ½Ð¸Ñ Ð½Ðµ Ð²Ñ‹Ð¿Ð¾Ð»Ð½ÐµÐ½Ð°
                (gd.dependency_type = 'influence_threshold' AND (p.influence < gd.required_influence_points OR p.influence IS NULL))
            )
        ) THEN true
        
        -- Ð˜Ð½Ð°Ñ‡Ðµ Ñ†ÐµÐ»ÑŒ ÑÐºÑ€Ñ‹Ñ‚Ð°
        ELSE false
    END AS is_visible,
    
    -- ÐžÐ¿Ñ€ÐµÐ´ÐµÐ»ÑÐµÐ¼, Ð·Ð°Ð±Ð»Ð¾ÐºÐ¸Ñ€Ð¾Ð²Ð°Ð½Ð° Ð»Ð¸ Ñ†ÐµÐ»ÑŒ (Ð²Ð¸Ð´Ð½Ð°, Ð½Ð¾ Ð½Ðµ Ð¼Ð¾Ð¶ÐµÑ‚ Ð±Ñ‹Ñ‚ÑŒ Ð²Ñ‹Ð¿Ð¾Ð»Ð½ÐµÐ½Ð°)
    CASE 
        -- Ð•ÑÐ»Ð¸ Ð½ÐµÑ‚ Ð·Ð°Ð²Ð¸ÑÐ¸Ð¼Ð¾ÑÑ‚ÐµÐ¹ - Ñ†ÐµÐ»ÑŒ Ð´Ð¾ÑÑ‚ÑƒÐ¿Ð½Ð°
        WHEN NOT EXISTS (
            SELECT 1 FROM goal_dependencies gd WHERE gd.goal_id = g.id
        ) THEN false
        
        -- ÐŸÑ€Ð¾Ð²ÐµÑ€ÑÐµÐ¼, ÐµÑÑ‚ÑŒ Ð»Ð¸ Ð½ÐµÐ²Ñ‹Ð¿Ð¾Ð»Ð½ÐµÐ½Ð½Ñ‹Ðµ Ð¸ Ð½ÐµÑ€Ð°Ð·Ð±Ð»Ð¾ÐºÐ¸Ñ€Ð¾Ð²Ð°Ð½Ð½Ñ‹Ðµ Ð·Ð°Ð²Ð¸ÑÐ¸Ð¼Ð¾ÑÑ‚Ð¸
        WHEN EXISTS (
            SELECT 1 FROM goal_dependencies gd
            LEFT JOIN goals rg ON gd.required_goal_id = rg.id
            LEFT JOIN players p ON gd.influence_player_id = p.id
            LEFT JOIN goal_dependency_unlocks gdu ON gd.id = gdu.dependency_id AND gd.goal_id = gdu.goal_id
            WHERE gd.goal_id = g.id 
            AND gdu.id IS NULL  -- Ð—Ð°Ð²Ð¸ÑÐ¸Ð¼Ð¾ÑÑ‚ÑŒ ÐµÑ‰Ñ‘ Ð½Ðµ Ñ€Ð°Ð·Ð±Ð»Ð¾ÐºÐ¸Ñ€Ð¾Ð²Ð°Ð½Ð°
            AND (
                -- Ð—Ð°Ð²Ð¸ÑÐ¸Ð¼Ð¾ÑÑ‚ÑŒ Ð¾Ñ‚ Ñ†ÐµÐ»Ð¸ Ð½Ðµ Ð²Ñ‹Ð¿Ð¾Ð»Ð½ÐµÐ½Ð°
                (gd.dependency_type = 'goal_completion' AND (rg.is_completed = false OR rg.is_completed IS NULL))
                OR
                -- Ð—Ð°Ð²Ð¸ÑÐ¸Ð¼Ð¾ÑÑ‚ÑŒ Ð¾Ñ‚ Ð¾Ñ‡ÐºÐ¾Ð² Ð²Ð»Ð¸ÑÐ½Ð¸Ñ Ð½Ðµ Ð²Ñ‹Ð¿Ð¾Ð»Ð½ÐµÐ½Ð°
                (gd.dependency_type = 'influence_threshold' AND (p.influence < gd.required_influence_points OR p.influence IS NULL))
            )
        ) THEN true
        
        -- Ð’ÑÐµ ÑƒÑÐ»Ð¾Ð²Ð¸Ñ Ð²Ñ‹Ð¿Ð¾Ð»Ð½ÐµÐ½Ñ‹ Ð¸Ð»Ð¸ Ñ€Ð°Ð·Ð±Ð»Ð¾ÐºÐ¸Ñ€Ð¾Ð²Ð°Ð½Ñ‹ - Ñ†ÐµÐ»ÑŒ Ð´Ð¾ÑÑ‚ÑƒÐ¿Ð½Ð°
        ELSE false
    END AS is_locked
FROM goals g
WHERE g.goal_type = 'personal';

-- ÐŸÑ€ÐµÐ´ÑÑ‚Ð°Ð²Ð»ÐµÐ½Ð¸Ðµ Ð´Ð»Ñ Ð°ÐºÑ‚Ð¸Ð²Ð½Ñ‹Ñ… Ð´Ð¾Ð³Ð¾Ð²Ð¾Ñ€Ð¾Ð²
CREATE OR REPLACE VIEW active_contracts AS
SELECT 
    c.*,
    customer.character_name AS customer_name,
    executor.character_name AS executor_name,
    customer_faction.name AS customer_faction_name
FROM contracts c
JOIN players customer ON c.customer_player_id = customer.id
JOIN players executor ON c.executor_player_id = executor.id
LEFT JOIN factions customer_faction ON c.customer_faction_id = customer_faction.id
WHERE c.status IN ('pending', 'signed') AND (c.expires_at IS NULL OR c.expires_at > CURRENT_TIMESTAMP);

-- Ð¡Ñ‡ÐµÑ‚Ñ‡Ð¸Ðº Ð²Ñ‹Ð¿Ð¾Ð»Ð½ÐµÐ½Ð½Ñ‹Ñ… Ð·Ð°Ð´Ð°Ñ‡ Ð¿Ð¾ Ð¸Ð³Ñ€Ð¾ÐºÐ°Ð¼
CREATE OR REPLACE VIEW player_tasks_stats AS
SELECT 
    player_id,
    COUNT(*) AS total_tasks,
    COUNT(CASE WHEN is_completed = true THEN 1 END) AS completed_tasks,
    COUNT(CASE WHEN is_completed = false THEN 1 END) AS pending_tasks
FROM tasks
GROUP BY player_id;

-- ÐÐºÑ‚Ð¸Ð²Ð½Ñ‹Ðµ Ñ€Ð°ÑƒÐ½Ð´Ñ‹ Ð³Ð¾Ð½ÐºÐ¸
CREATE OR REPLACE VIEW active_goal_race_rounds AS
SELECT 
    grr.*,
    COUNT(DISTINCT grrp.player_id) AS participants_count,
    COUNT(DISTINCT CASE WHEN grrg.is_accessible = true THEN grrg.id END) AS accessible_goals_count
FROM goal_race_rounds grr
LEFT JOIN goal_race_round_participants grrp ON grr.id = grrp.round_id
LEFT JOIN goal_race_round_goals grrg ON grr.id = grrg.round_id
WHERE grr.status = 'active'
GROUP BY grr.id;

-- ÐŸÑ€Ð¾Ð³Ñ€ÐµÑÑ Ð¸Ð³Ñ€Ð¾ÐºÐ° Ð² Ñ‚ÐµÐºÑƒÑ‰Ð¸Ñ… Ð³Ð¾Ð½ÐºÐ°Ñ…
CREATE OR REPLACE VIEW player_race_progress AS
SELECT 
    grrp.player_id,
    grrp.round_id,
    grr.round_number,
    grr.status AS round_status,
    COUNT(grrg.id) AS total_goals,
    COUNT(CASE WHEN g.is_completed = true THEN 1 END) AS completed_goals,
    COUNT(CASE WHEN grrg.is_accessible = true THEN 1 END) AS accessible_goals
FROM goal_race_round_participants grrp
JOIN goal_race_rounds grr ON grrp.round_id = grr.id
LEFT JOIN goal_race_round_goals grrg ON grr.id = grrg.round_id AND grrg.assigned_player_id = grrp.player_id
LEFT JOIN goals g ON grrg.goal_id = g.id
GROUP BY grrp.player_id, grrp.round_id, grr.round_number, grr.status;

-- ============================================
-- ÐšÐžÐœÐœÐ•ÐÐ¢ÐÐ Ð˜Ð˜ Ðš Ð¢ÐÐ‘Ð›Ð˜Ð¦ÐÐœ
-- ============================================

COMMENT ON TABLE factions IS 'Ð¤Ñ€Ð°ÐºÑ†Ð¸Ð¸ Ð² Ð¸Ð³Ñ€Ðµ (Ð´Ð²Ð¾Ñ€ÐµÑ†, Ð¼Ð°Ñ„Ð¸Ñ Ð¸ Ñ‚.Ð´.)';
COMMENT ON TABLE players IS 'Ð˜Ð³Ñ€Ð¾ÐºÐ¸ Ð¸ Ð¸Ñ… Ð¿ÐµÑ€ÑÐ¾Ð½Ð°Ð¶Ð¸';
COMMENT ON TABLE items IS 'ÐŸÑ€ÐµÐ´Ð¼ÐµÑ‚Ñ‹ Ð² Ð¸Ð³Ñ€Ðµ';
COMMENT ON TABLE effects IS 'Ð­Ñ„Ñ„ÐµÐºÑ‚Ñ‹, ÐºÐ¾Ñ‚Ð¾Ñ€Ñ‹Ðµ Ð¼Ð¾Ð³ÑƒÑ‚ Ð¸Ð¼ÐµÑ‚ÑŒ Ð¿Ñ€ÐµÐ´Ð¼ÐµÑ‚Ñ‹';
COMMENT ON TABLE abilities IS 'Ð£Ð½Ð¸ÐºÐ°Ð»ÑŒÐ½Ñ‹Ðµ ÑÐ¿Ð¾ÑÐ¾Ð±Ð½Ð¾ÑÑ‚Ð¸ Ð¸Ð³Ñ€Ð¾ÐºÐ¾Ð²';
COMMENT ON TABLE goals IS 'Ð›Ð¸Ñ‡Ð½Ñ‹Ðµ Ð¸ Ñ„Ñ€Ð°ÐºÑ†Ð¸Ð¾Ð½Ð½Ñ‹Ðµ Ñ†ÐµÐ»Ð¸';
COMMENT ON TABLE contracts IS 'Ð”Ð¾Ð³Ð¾Ð²Ð¾Ñ€Ñ‹ Ð¼ÐµÐ¶Ð´Ñƒ Ð¸Ð³Ñ€Ð¾ÐºÐ°Ð¼Ð¸';
COMMENT ON TABLE debt_receipts IS 'Ð”Ð¾Ð»Ð³Ð¾Ð²Ñ‹Ðµ Ñ€Ð°ÑÐ¿Ð¸ÑÐºÐ¸';
COMMENT ON TABLE money_transactions IS 'Ð˜ÑÑ‚Ð¾Ñ€Ð¸Ñ Ð´ÐµÐ½ÐµÐ¶Ð½Ñ‹Ñ… Ñ‚Ñ€Ð°Ð½Ð·Ð°ÐºÑ†Ð¸Ð¹ Ð´Ð»Ñ Ð¾Ñ‚ÑÐ»ÐµÐ¶Ð¸Ð²Ð°Ð½Ð¸Ñ Ð°Ð´Ð¼Ð¸Ð½Ð°Ð¼Ð¸';
COMMENT ON TABLE item_transactions IS 'Ð˜ÑÑ‚Ð¾Ñ€Ð¸Ñ Ð¿ÐµÑ€ÐµÐ´Ð°Ñ‡Ð¸ Ð¿Ñ€ÐµÐ´Ð¼ÐµÑ‚Ð¾Ð²';
COMMENT ON TABLE influence_transactions IS 'Ð˜ÑÑ‚Ð¾Ñ€Ð¸Ñ Ð¸Ð·Ð¼ÐµÐ½ÐµÐ½Ð¸Ñ Ð¾Ñ‡ÐºÐ¾Ð² Ð²Ð»Ð¸ÑÐ½Ð¸Ñ';
COMMENT ON TABLE tasks IS 'Ð—Ð°Ð´Ð°Ñ‡Ð¸ Ð¸Ð³Ñ€Ð¾ÐºÐ¾Ð² (Ð¾Ñ‚Ð»Ð¸Ñ‡Ð°ÑŽÑ‚ÑÑ Ð¾Ñ‚ Ñ†ÐµÐ»ÐµÐ¹)';
COMMENT ON TABLE goal_race_triggers IS 'Ð£ÑÐ»Ð¾Ð²Ð¸Ñ Ð´Ð»Ñ Ð·Ð°Ð¿ÑƒÑÐºÐ° Ð³Ð¾Ð½Ð¾Ðº Ñ†ÐµÐ»ÐµÐ¹';
COMMENT ON TABLE goal_race_rounds IS 'Ð Ð°ÑƒÐ½Ð´Ñ‹ Ð³Ð¾Ð½ÐºÐ¸ Ñ†ÐµÐ»ÐµÐ¹';
COMMENT ON TABLE goal_race_predefined_goals IS 'Ð—Ð°Ñ€Ð°Ð½ÐµÐµ ÑÐ¾Ð·Ð´Ð°Ð½Ð½Ñ‹Ðµ Ñ†ÐµÐ»Ð¸ Ð´Ð»Ñ Ð±ÑƒÐ´ÑƒÑ‰Ð¸Ñ… Ñ€Ð°ÑƒÐ½Ð´Ð¾Ð² Ð³Ð¾Ð½ÐºÐ¸';
COMMENT ON TABLE goal_race_round_goals IS 'ÐÐ°Ð·Ð½Ð°Ñ‡ÐµÐ½Ð¸Ðµ Ñ†ÐµÐ»ÐµÐ¹ Ð¸Ð³Ñ€Ð¾ÐºÐ°Ð¼ Ð² Ñ€Ð°Ð¼ÐºÐ°Ñ… Ñ€Ð°ÑƒÐ½Ð´Ð°';
COMMENT ON COLUMN goal_race_round_goals.is_accessible IS 'Ð¡Ñ‚Ð°Ð½Ð¾Ð²Ð¸Ñ‚ÑÑ false ÐºÐ¾Ð³Ð´Ð° Ð´Ñ€ÑƒÐ³Ð¾Ð¹ Ð¸Ð³Ñ€Ð¾Ðº Ð·Ð°Ð²ÐµÑ€ÑˆÐ°ÐµÑ‚ Ñ€Ð°ÑƒÐ½Ð´';
COMMENT ON COLUMN goal_race_rounds.round_number IS 'ÐŸÐ¾Ñ€ÑÐ´ÐºÐ¾Ð²Ñ‹Ð¹ Ð½Ð¾Ð¼ÐµÑ€ Ñ€Ð°ÑƒÐ½Ð´Ð° Ð² Ñ€Ð°Ð¼ÐºÐ°Ñ… Ð¾Ð´Ð½Ð¾Ð¹ Ð³Ð¾Ð½ÐºÐ¸';
COMMENT ON COLUMN goal_race_rounds.status IS 'pending - ÑÐ¾Ð·Ð´Ð°Ð½, Ð½Ð¾ Ð½Ðµ Ð½Ð°Ñ‡Ð°Ñ‚; active - Ñ‚ÐµÐºÑƒÑ‰Ð¸Ð¹; completed - Ð·Ð°Ð²ÐµÑ€ÑˆÐµÐ½; cancelled - Ð¾Ñ‚Ð¼ÐµÐ½ÐµÐ½';

-- Ð¢Ð°Ð±Ð»Ð¸Ñ†Ð° Ð¿Ð¾Ð»ÑŒÐ·Ð¾Ð²Ð°Ñ‚ÐµÐ»ÐµÐ¹ Ð´Ð»Ñ Ð°Ð²Ñ‚Ð¾Ñ€Ð¸Ð·Ð°Ñ†Ð¸Ð¸
CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    username VARCHAR(255) NOT NULL UNIQUE,
    password VARCHAR(255) NOT NULL,
    player_id INTEGER REFERENCES players(id) ON DELETE SET NULL,
    is_admin BOOLEAN DEFAULT false
);

-- Ð˜Ð½Ð´ÐµÐºÑ Ð´Ð»Ñ Ð±Ñ‹ÑÑ‚Ñ€Ð¾Ð³Ð¾ Ð¿Ð¾Ð¸ÑÐºÐ° Ð¿Ð¾ username
CREATE INDEX idx_users_username ON users(username);

COMMENT ON TABLE users IS 'ÐŸÐ¾Ð»ÑŒÐ·Ð¾Ð²Ð°Ñ‚ÐµÐ»Ð¸ ÑÐ¸ÑÑ‚ÐµÐ¼Ñ‹ Ð´Ð»Ñ Ð°Ð²Ñ‚Ð¾Ñ€Ð¸Ð·Ð°Ñ†Ð¸Ð¸';
COMMENT ON COLUMN users.player_id IS 'Ð¡Ð²ÑÐ·ÑŒ Ñ Ð¿ÐµÑ€ÑÐ¾Ð½Ð°Ð¶ÐµÐ¼ Ð¸Ð³Ñ€Ð¾ÐºÐ° (Ð¼Ð¾Ð¶ÐµÑ‚ Ð±Ñ‹Ñ‚ÑŒ NULL Ð´Ð»Ñ Ð°Ð´Ð¼Ð¸Ð½Ð¾Ð² Ð±ÐµÐ· Ð¿ÐµÑ€ÑÐ¾Ð½Ð°Ð¶Ð°)';
COMMENT ON COLUMN users.is_admin IS 'Ð¤Ð»Ð°Ð³ Ð°Ð´Ð¼Ð¸Ð½Ð¸ÑÑ‚Ñ€Ð°Ñ‚Ð¾Ñ€Ð° ÑÐ¸ÑÑ‚ÐµÐ¼Ñ‹';
-- Ð”Ð¾Ð¿Ð¾Ð»Ð½Ð¸Ñ‚ÐµÐ»ÑŒÐ½Ñ‹Ðµ Ð¸Ð½Ð´ÐµÐºÑÑ‹ Ð´Ð»Ñ Ð½Ð¾Ð²Ñ‹Ñ… Ñ‚Ð°Ð±Ð»Ð¸Ñ† Ð·Ð°Ð²Ð¸ÑÐ¸Ð¼Ð¾ÑÑ‚ÐµÐ¹
CREATE INDEX idx_goal_dependencies_goal ON goal_dependencies(goal_id);
CREATE INDEX idx_goal_dependencies_required_goal ON goal_dependencies(required_goal_id);
CREATE INDEX idx_goal_dependencies_influence_player ON goal_dependencies(influence_player_id);
CREATE INDEX idx_goal_dependencies_type ON goal_dependencies(dependency_type);

CREATE INDEX idx_goal_dependency_unlocks_goal ON goal_dependency_unlocks(goal_id);
CREATE INDEX idx_goal_dependency_unlocks_dependency ON goal_dependency_unlocks(dependency_id);
CREATE INDEX idx_goal_dependency_unlocks_player ON goal_dependency_unlocks(player_id);

-- ============================================
-- Ð¢Ð Ð˜Ð“Ð“Ð•Ð Ð« Ð”Ð›Ð¯ ÐÐ’Ð¢ÐžÐœÐÐ¢Ð˜Ð§Ð•Ð¡ÐšÐžÐ™ Ð ÐÐ—Ð‘Ð›ÐžÐšÐ˜Ð ÐžÐ’ÐšÐ˜
-- ============================================

-- Ð¤ÑƒÐ½ÐºÑ†Ð¸Ñ Ð´Ð»Ñ Ð°Ð²Ñ‚Ð¾Ð¼Ð°Ñ‚Ð¸Ñ‡ÐµÑÐºÐ¾Ð¹ Ñ€Ð°Ð·Ð±Ð»Ð¾ÐºÐ¸Ñ€Ð¾Ð²ÐºÐ¸ Ð·Ð°Ð²Ð¸ÑÐ¸Ð¼Ð¾ÑÑ‚ÐµÐ¹ Ð¿Ñ€Ð¸ Ð¸Ð·Ð¼ÐµÐ½ÐµÐ½Ð¸Ð¸ Ð²Ð»Ð¸ÑÐ½Ð¸Ñ
CREATE OR REPLACE FUNCTION unlock_goal_dependencies_on_influence_change()
RETURNS TRIGGER AS $$
BEGIN
    -- ÐšÐ¾Ð³Ð´Ð° Ñƒ Ð¸Ð³Ñ€Ð¾ÐºÐ° Ð¼ÐµÐ½ÑÐµÑ‚ÑÑ influence, Ð¿Ñ€Ð¾Ð²ÐµÑ€ÑÐµÐ¼ Ð²ÑÐµ Ð·Ð°Ð²Ð¸ÑÐ¸Ð¼Ð¾ÑÑ‚Ð¸ Ð¾Ñ‚ ÐµÐ³Ð¾ Ð²Ð»Ð¸ÑÐ½Ð¸Ñ
    INSERT INTO goal_dependency_unlocks (goal_id, dependency_id, player_id)
    SELECT 
        gd.goal_id,
        gd.id,
        g.player_id
    FROM goal_dependencies gd
    JOIN goals g ON gd.goal_id = g.id
    LEFT JOIN goal_dependency_unlocks gdu ON gd.id = gdu.dependency_id AND gd.goal_id = gdu.goal_id
    WHERE gd.dependency_type = 'influence_threshold'
        AND gd.influence_player_id = NEW.id
        AND NEW.influence >= gd.required_influence_points
        AND gdu.id IS NULL  -- Ð•Ñ‰Ñ‘ Ð½Ðµ Ñ€Ð°Ð·Ð±Ð»Ð¾ÐºÐ¸Ñ€Ð¾Ð²Ð°Ð½Ð¾
    ON CONFLICT (goal_id, dependency_id) DO NOTHING;
    
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Ð¢Ñ€Ð¸Ð³Ð³ÐµÑ€ Ð´Ð»Ñ Ð°Ð²Ñ‚Ð¾Ð¼Ð°Ñ‚Ð¸Ñ‡ÐµÑÐºÐ¾Ð¹ Ñ€Ð°Ð·Ð±Ð»Ð¾ÐºÐ¸Ñ€Ð¾Ð²ÐºÐ¸ Ð¿Ñ€Ð¸ Ð¸Ð·Ð¼ÐµÐ½ÐµÐ½Ð¸Ð¸ Ð²Ð»Ð¸ÑÐ½Ð¸Ñ
DROP TRIGGER IF EXISTS trigger_unlock_on_influence_change ON players;
CREATE TRIGGER trigger_unlock_on_influence_change
    AFTER UPDATE OF influence ON players
    FOR EACH ROW
    WHEN (OLD.influence IS DISTINCT FROM NEW.influence)
    EXECUTE FUNCTION unlock_goal_dependencies_on_influence_change();

-- Ð¤ÑƒÐ½ÐºÑ†Ð¸Ñ Ð´Ð»Ñ Ð°Ð²Ñ‚Ð¾Ð¼Ð°Ñ‚Ð¸Ñ‡ÐµÑÐºÐ¾Ð¹ Ñ€Ð°Ð·Ð±Ð»Ð¾ÐºÐ¸Ñ€Ð¾Ð²ÐºÐ¸ Ð·Ð°Ð²Ð¸ÑÐ¸Ð¼Ð¾ÑÑ‚ÐµÐ¹ Ð¿Ñ€Ð¸ Ð²Ñ‹Ð¿Ð¾Ð»Ð½ÐµÐ½Ð¸Ð¸ Ñ†ÐµÐ»ÐµÐ¹
CREATE OR REPLACE FUNCTION unlock_goal_dependencies_on_goal_completion()
RETURNS TRIGGER AS $$
BEGIN
    -- ÐšÐ¾Ð³Ð´Ð° Ñ†ÐµÐ»ÑŒ Ð²Ñ‹Ð¿Ð¾Ð»Ð½ÑÐµÑ‚ÑÑ, Ñ€Ð°Ð·Ð±Ð»Ð¾ÐºÐ¸Ñ€ÑƒÐµÐ¼ Ð²ÑÐµ Ð·Ð°Ð²Ð¸ÑÐ¸Ð¼Ð¾ÑÑ‚Ð¸ Ð¾Ñ‚ Ð½ÐµÑ‘
    INSERT INTO goal_dependency_unlocks (goal_id, dependency_id, player_id)
    SELECT 
        gd.goal_id,
        gd.id,
        g.player_id
    FROM goal_dependencies gd
    JOIN goals g ON gd.goal_id = g.id
    LEFT JOIN goal_dependency_unlocks gdu ON gd.id = gdu.dependency_id AND gd.goal_id = gdu.goal_id
    WHERE gd.dependency_type = 'goal_completion'
        AND gd.required_goal_id = NEW.id
        AND NEW.is_completed = true
        AND gdu.id IS NULL  -- Ð•Ñ‰Ñ‘ Ð½Ðµ Ñ€Ð°Ð·Ð±Ð»Ð¾ÐºÐ¸Ñ€Ð¾Ð²Ð°Ð½Ð¾
    ON CONFLICT (goal_id, dependency_id) DO NOTHING;
    
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trigger_unlock_on_goal_completion ON goals;
CREATE TRIGGER trigger_unlock_on_goal_completion
    AFTER UPDATE OF is_completed ON goals
    FOR EACH ROW
    WHEN (OLD.is_completed IS DISTINCT FROM NEW.is_completed AND NEW.is_completed = true)
    EXECUTE FUNCTION unlock_goal_dependencies_on_goal_completion();