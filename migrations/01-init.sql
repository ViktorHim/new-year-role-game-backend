-- migrations/01-init.sql

-- ============================================
-- СХЕМА БАЗЫ ДАННЫХ ДЛЯ РОЛЕВОЙ ИГРЫ
-- ============================================

-- ============================================
-- ОСНОВНЫЕ ТАБЛИЦЫ
-- ============================================

-- Фракции¸
CREATE TABLE IF NOT EXISTS factions (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL UNIQUE,
    description TEXT,
    faction_influence INTEGER DEFAULT 0, -- очки самой фракции (не игроков)
    is_composition_visible_to_all BOOLEAN DEFAULT false,
    leader_player_id INTEGER -- будет добавлен FK после создания players
);

-- Игроки¸
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
    avatar TEXT -- изображение в base64
);

-- информация, доступная игроку о других
CREATE TABLE IF NOT EXISTS info_about_other_players (
    id SERIAL PRIMARY KEY,
    player_id INTEGER REFERENCES players(id) on DELETE SET NULL,
    description TEXT
);

-- Добавляем FK для лидера фракции после создания таблицы players
ALTER TABLE factions 
ADD CONSTRAINT fk_leader_player 
FOREIGN KEY (leader_player_id) REFERENCES players(id) ON DELETE SET NULL;

-- Ð’ÐµÑ‰Ð¸ (Ð¿Ñ€ÐµÐ´Ð¼ÐµÑ‚Ñ‹)
CREATE TABLE IF NOT EXISTS items (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Вещи (предметы)
CREATE TABLE IF NOT EXISTS effects (
    id SERIAL PRIMARY KEY,
    description TEXT,
    effect_type VARCHAR(20) NOT NULL, -- 'generate_money', 'generate_influence', 'spawn_item'
    -- Для генерации денег/влияния
    generated_resource VARCHAR(20), -- 'money', 'influence'
    operation VARCHAR(10) DEFAULT 'add', -- 'add', 'mul', 'sub', 'div'
    value INTEGER,
    -- Для создания предметов
    spawned_item_id INTEGER REFERENCES items(id) ON DELETE CASCADE,
    -- Период действия
    period_seconds INTEGER NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    CHECK (
        (effect_type IN ('generate_money', 'generate_influence') AND generated_resource IS NOT NULL AND value IS NOT NULL AND spawned_item_id IS NULL) OR
        (effect_type = 'spawn_item' AND spawned_item_id IS NOT NULL AND generated_resource IS NULL)
    )
);

-- Связь вещей и эффектов (одна вещь может иметь несколько эффектов)
CREATE TABLE IF NOT EXISTS item_effects (
    item_id INTEGER REFERENCES items(id) ON DELETE CASCADE,
    effect_id INTEGER REFERENCES effects(id) ON DELETE CASCADE,
    PRIMARY KEY (item_id, effect_id)
);

-- Инвентарь игроков
CREATE TABLE IF NOT EXISTS player_items (
    id SERIAL PRIMARY KEY,
    player_id INTEGER REFERENCES players(id) ON DELETE CASCADE,
    item_id INTEGER REFERENCES items(id) ON DELETE CASCADE,
    acquired_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(player_id, item_id)
);

-- Отслеживание последнего выполнения эффектов вещей
CREATE TABLE IF NOT EXISTS item_effect_executions (
    id SERIAL PRIMARY KEY,
    player_id INTEGER REFERENCES players(id) ON DELETE CASCADE,
    item_id INTEGER REFERENCES items(id) ON DELETE CASCADE,
    effect_id INTEGER REFERENCES effects(id) ON DELETE CASCADE,
    last_executed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(player_id, item_id, effect_id)
);

-- ============================================
-- СПОСОБНОСТИ
-- ============================================

-- Уникальные способности
CREATE TABLE IF NOT EXISTS abilities (
    id SERIAL PRIMARY KEY,
    player_id INTEGER REFERENCES players(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    ability_type VARCHAR(50) NOT NULL, -- 'reveal_info', 'add_influence', 'transfer_influence'
    cooldown_minutes INTEGER DEFAULT NULL,
    start_delay_minutes INTEGER DEFAULT NULL, -- задержка от начала игры
    required_influence_points INTEGER DEFAULT NULL, -- минимальное количество очков влияния для разблокировки
    is_unlocked BOOLEAN DEFAULT true, -- была ли способность разблокирована (после разблокировки остается доступной всегда)
    -- Для способности начисления влияния другому игроку (add_influence)
    influence_points_to_add INTEGER,
    -- Для способности снятия влияния у другого игрока и начисления себе (transfer_influence)
    influence_points_to_remove INTEGER, -- сколько снять у целевого игрока
    influence_points_to_self INTEGER, -- сколько начислить себе
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

-- История использования способностей (для отслеживания cooldown)
CREATE TABLE IF NOT EXISTS ability_usage (
    id SERIAL PRIMARY KEY,
    player_id INTEGER REFERENCES players(id) ON DELETE CASCADE,
    ability_id INTEGER REFERENCES abilities(id) ON DELETE CASCADE,
    target_player_id INTEGER REFERENCES players(id) ON DELETE SET NULL, -- для способностей, направленных на других игроков
    info_category VARCHAR(20), -- 'faction', 'goal', 'item' (для reveal_info)
    used_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- История раскрытой информации
CREATE TABLE IF NOT EXISTS revealed_info (
    id SERIAL PRIMARY KEY,
    revealer_player_id INTEGER REFERENCES players(id) ON DELETE CASCADE,
    target_player_id INTEGER REFERENCES players(id) ON DELETE CASCADE,
    info_type VARCHAR(20) NOT NULL, -- 'faction', 'goal', 'item'
    revealed_data JSONB, -- JSON с раскрытой информацией
    revealed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    ability_usage_id INTEGER REFERENCES ability_usage(id) ON DELETE SET NULL
);

-- ============================================
-- ЦЕЛИ
-- ============================================

-- Цели (личные и фракционные)
CREATE TABLE IF NOT EXISTS goals (
    id SERIAL PRIMARY KEY,
    title VARCHAR(255) NOT NULL,
    description TEXT,
    goal_type VARCHAR(20) NOT NULL, -- 'personal', 'faction'
    influence_points_reward INTEGER DEFAULT 0,
    -- Для личных целей
    player_id INTEGER REFERENCES players(id) ON DELETE CASCADE,
    -- Для фракционных целей
    faction_id INTEGER REFERENCES factions(id) ON DELETE CASCADE,
    is_completed BOOLEAN DEFAULT false,
    completed_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    CHECK (
        (goal_type = 'personal' AND player_id IS NOT NULL AND faction_id IS NULL) OR
        (goal_type = 'faction' AND faction_id IS NOT NULL AND player_id IS NULL)
    )
);


-- Зависимости целей друг от друга (скрытые цели)
-- ОБНОВЛЕНО: Теперь поддерживает зависимость от влияния других игроков
CREATE TABLE IF NOT EXISTS goal_dependencies (
    id SERIAL PRIMARY KEY,
    goal_id INTEGER REFERENCES goals(id) ON DELETE CASCADE, -- эта цель зависит от...
    
    -- Тип зависимости
    dependency_type VARCHAR(30) NOT NULL, -- 'goal_completion' или 'influence_threshold'
    
    -- Для зависимости от выполнения другой цели
    required_goal_id INTEGER REFERENCES goals(id) ON DELETE CASCADE,
    
    -- Для зависимости от очков влияния другого игрока
    influence_player_id INTEGER REFERENCES players(id) ON DELETE CASCADE,
    required_influence_points INTEGER,
    
    -- Видимость до выполнения условия
    is_visible_before_completion BOOLEAN DEFAULT false, -- false = полностью скрыта; true = видна, но заблокирована
    
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    -- Проверки целостности данных
    CHECK (
        -- Для типа 'goal_completion' должен быть указан required_goal_id
        (dependency_type = 'goal_completion' AND 
         required_goal_id IS NOT NULL AND 
         influence_player_id IS NULL AND 
         required_influence_points IS NULL) OR
        -- Для типа 'influence_threshold' должны быть указаны influence_player_id и required_influence_points
        (dependency_type = 'influence_threshold' AND 
         required_goal_id IS NULL AND 
         influence_player_id IS NOT NULL AND 
         required_influence_points IS NOT NULL AND
         required_influence_points > 0)
    ),
    
    -- Цель не может зависеть от самой себя
    CHECK (goal_id != required_goal_id),
    
    -- Уникальность зависимостей
    UNIQUE(goal_id, dependency_type, required_goal_id),
    UNIQUE(goal_id, dependency_type, influence_player_id)
);

-- История разблокировок зависимостей целей
-- Когда зависимость разблокируется (цель выполнена или порог влияния достигнут),
-- запись добавляется в эту таблицу и остаётся там навсегда
-- Это гарантирует, что цель остаётся доступной даже если условие перестало выполняться
CREATE TABLE IF NOT EXISTS goal_dependency_unlocks (
    id SERIAL PRIMARY KEY,
    goal_id INTEGER REFERENCES goals(id) ON DELETE CASCADE,
    dependency_id INTEGER REFERENCES goal_dependencies(id) ON DELETE CASCADE,
    player_id INTEGER REFERENCES players(id) ON DELETE CASCADE, -- владелец цели
    unlocked_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    -- Одна зависимость может быть разблокирована только один раз для одной цели
    UNIQUE(goal_id, dependency_id)
);

-- История выполнения целей (для отслеживания начисления/снятия очков влияния)
CREATE TABLE IF NOT EXISTS goal_completion_history (
    id SERIAL PRIMARY KEY,
    goal_id INTEGER REFERENCES goals(id) ON DELETE CASCADE,
    player_id INTEGER REFERENCES players(id) ON DELETE CASCADE, -- кто отметил цель
    action VARCHAR(20) NOT NULL, -- 'completed', 'uncompleted'
    influence_change INTEGER NOT NULL, -- изменение влияния
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- ============================================
-- ЗАДАЧИ И ГОНКА ЦЕЛЕЙ
-- ============================================

-- Задачи игроков (отличаются от целей)
CREATE TABLE IF NOT EXISTS tasks (
    id SERIAL PRIMARY KEY,
    player_id INTEGER REFERENCES players(id) ON DELETE CASCADE,
    title VARCHAR(255) NOT NULL,
    description TEXT,
    is_completed BOOLEAN DEFAULT false,
    completed_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- История выполнения задач
CREATE TABLE IF NOT EXISTS task_completion_history (
    id SERIAL PRIMARY KEY,
    task_id INTEGER REFERENCES tasks(id) ON DELETE CASCADE,
    player_id INTEGER REFERENCES players(id) ON DELETE CASCADE,
    action VARCHAR(20) NOT NULL, -- 'completed', 'uncompleted'
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Условия для запуска гонки целей
CREATE TABLE IF NOT EXISTS goal_race_triggers (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    required_tasks_count INTEGER NOT NULL CHECK (required_tasks_count > 0),
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Игроки, участвующие в гонке при срабатывании триггера
CREATE TABLE IF NOT EXISTS goal_race_trigger_participants (
    id SERIAL PRIMARY KEY,
    trigger_id INTEGER REFERENCES goal_race_triggers(id) ON DELETE CASCADE,
    player_id INTEGER REFERENCES players(id) ON DELETE CASCADE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(trigger_id, player_id)
);

-- Раунды гонки целей
CREATE TABLE IF NOT EXISTS goal_race_rounds (
    id SERIAL PRIMARY KEY,
    trigger_id INTEGER REFERENCES goal_race_triggers(id) ON DELETE SET NULL,
    round_number INTEGER NOT NULL DEFAULT 1, -- номер раунда в рамках одной гонки
    status VARCHAR(20) DEFAULT 'pending', -- 'pending', 'active', 'completed', 'cancelled'
    started_at TIMESTAMP,
    completed_at TIMESTAMP,
    winner_player_id INTEGER REFERENCES players(id) ON DELETE SET NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Участники конкретного раунда
CREATE TABLE IF NOT EXISTS goal_race_round_participants (
    id SERIAL PRIMARY KEY,
    round_id INTEGER REFERENCES goal_race_rounds(id) ON DELETE CASCADE,
    player_id INTEGER REFERENCES players(id) ON DELETE CASCADE,
    joined_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(round_id, player_id)
);

-- Предопределенные цели для раундов гонки
-- Админ создает эти цели ЗАРАНЕЕ, до запуска гонки
CREATE TABLE IF NOT EXISTS goal_race_predefined_goals (
    id SERIAL PRIMARY KEY,
    trigger_id INTEGER REFERENCES goal_race_triggers(id) ON DELETE CASCADE,
    round_number INTEGER NOT NULL, -- для какого раунда эта цель
    player_id INTEGER REFERENCES players(id) ON DELETE CASCADE, -- кому назначена цель
    title VARCHAR(255) NOT NULL,
    description TEXT,
    influence_points_reward INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(trigger_id, round_number, player_id, title) -- один игрок не может получить одинаковую цель в раунде дважды
);

-- Связь целей с раундами гонки (создается при активации раунда)
CREATE TABLE IF NOT EXISTS goal_race_round_goals (
    id SERIAL PRIMARY KEY,
    round_id INTEGER REFERENCES goal_race_rounds(id) ON DELETE CASCADE,
    goal_id INTEGER REFERENCES goals(id) ON DELETE CASCADE,
    assigned_player_id INTEGER REFERENCES players(id) ON DELETE CASCADE,
    is_accessible BOOLEAN DEFAULT true, -- false когда раунд завершается
    assigned_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    became_inaccessible_at TIMESTAMP,
    UNIQUE(round_id, goal_id),
    UNIQUE(round_id, assigned_player_id, goal_id)
);

-- ============================================
-- ДОГОВОРЫ
-- ============================================

-- Договоры между игроками
CREATE TABLE IF NOT EXISTS contracts (
    id SERIAL PRIMARY KEY,
    contract_type VARCHAR(20) NOT NULL, -- 'type1', 'type2'
    customer_player_id INTEGER REFERENCES players(id) ON DELETE CASCADE,
    executor_player_id INTEGER REFERENCES players(id) ON DELETE CASCADE,
    customer_faction_id INTEGER REFERENCES factions(id) ON DELETE SET NULL, -- фракция заказчика на момент подписания
    status VARCHAR(20) DEFAULT 'pending', -- 'pending', 'signed', 'completed', 'terminated'
    duration_seconds INTEGER NOT NULL,
    money_reward_customer INTEGER DEFAULT 0, -- деньги для заказчика
    money_reward_executor INTEGER DEFAULT 0, -- деньги для исполнителя
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    signed_at TIMESTAMP,
    expires_at TIMESTAMP,
    completed_at TIMESTAMP,
    terminated_at TIMESTAMP,
    CHECK (customer_player_id != executor_player_id)
);

-- Настройки награды для контракта типа 1 (вещи по фракциям)
CREATE TABLE IF NOT EXISTS contract_type1_settings (
    id SERIAL PRIMARY KEY,
    faction_id INTEGER REFERENCES factions(id) ON DELETE CASCADE,
    customer_item_reward_id INTEGER REFERENCES items(id) ON DELETE SET NULL,
    UNIQUE(faction_id)
);

-- Настройки штрафов за нарушение договора
CREATE TABLE IF NOT EXISTS contract_penalty_settings (
    id SERIAL PRIMARY KEY,
    money_penalty INTEGER DEFAULT 0,
    influence_penalty INTEGER DEFAULT 0
);

-- История штрафов по договорам
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
-- ДОЛГОВЫЕ РАСПИСКИ
-- ============================================

-- Долговые расписки
CREATE TABLE IF NOT EXISTS debt_receipts (
    id SERIAL PRIMARY KEY,    
    lender_player_id INTEGER REFERENCES players(id) ON DELETE CASCADE, -- кредитор
    borrower_player_id INTEGER REFERENCES players(id) ON DELETE CASCADE, -- заемщик
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

-- Настройки штрафов для долговых расписок
CREATE TABLE IF NOT EXISTS debt_penalty_settings (
    id SERIAL PRIMARY KEY,
    penalty_influence_points INTEGER DEFAULT 0
);

-- ============================================
-- ТРАНЗАКЦИИ
-- ============================================

-- Денежные транзакции (для отслеживания админами)
CREATE TABLE IF NOT EXISTS money_transactions (
    id SERIAL PRIMARY KEY,
    from_player_id INTEGER REFERENCES players(id) ON DELETE SET NULL,
    to_player_id INTEGER REFERENCES players(id) ON DELETE SET NULL,
    amount INTEGER NOT NULL,
    transaction_type VARCHAR(50) NOT NULL, -- 'transfer', 'contract', 'debt', 'penalty', 'item_effect'
    reference_id INTEGER, -- ID связанного договора, долга и т.д.
    reference_type VARCHAR(50), -- 'contract', 'debt_receipt', 'effect'
    description TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Транзакции предметов
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

-- Транзакции влияния
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
-- ИГРОВЫЕ НАСТРОЙКИ
-- ============================================

-- Общие настройки игры
CREATE TABLE IF NOT EXISTS game_settings (
    id SERIAL PRIMARY KEY,
    setting_key VARCHAR(100) NOT NULL UNIQUE,
    setting_value TEXT,
    description TEXT,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Временные метки игры
CREATE TABLE IF NOT EXISTS game_timeline (
    id SERIAL PRIMARY KEY,
    game_started_at TIMESTAMP,
    game_ended_at TIMESTAMP
);

-- ============================================
-- ИНДЕКСЫ ДЛЯ ПРОИЗВОДИТЕЛЬНОСТИ
-- ============================================

-- Индексы для частых запросов
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

-- Индексы для задач и гонки целей
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
-- ПРЕДСТАВЛЕНИЯ (VIEWS)
-- ============================================

-- Представление для подсчета общего влияния фракции
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


-- Представление для видимых целей игрока
-- ОБНОВЛЕНО: Учитывает разблокировки из goal_dependency_unlocks
CREATE OR REPLACE VIEW player_visible_goals AS
SELECT 
    g.id,
    g.title,
    g.description,
    g.player_id,
    g.influence_points_reward,
    g.is_completed,
    -- Определяем видимость цели
    CASE 
        -- Если нет зависимостей - цель видна
        WHEN NOT EXISTS (
            SELECT 1 FROM goal_dependencies gd WHERE gd.goal_id = g.id
        ) THEN true
        
        -- Если есть зависимость с is_visible_before_completion = true - цель видна (но может быть заблокирована)
        WHEN EXISTS (
            SELECT 1 FROM goal_dependencies gd 
            WHERE gd.goal_id = g.id 
            AND gd.is_visible_before_completion = true
        ) THEN true
        
        -- Проверяем, выполнены ли ВСЕ условия зависимости (с учетом unlocks!)
        WHEN NOT EXISTS (
            SELECT 1 FROM goal_dependencies gd
            LEFT JOIN goals rg ON gd.required_goal_id = rg.id
            LEFT JOIN players p ON gd.influence_player_id = p.id
            LEFT JOIN goal_dependency_unlocks gdu ON gd.id = gdu.dependency_id AND gd.goal_id = gdu.goal_id
            WHERE gd.goal_id = g.id 
            AND gdu.id IS NULL  -- Зависимость ещё не разблокирована
            AND (
                -- Зависимость от цели не выполнена
                (gd.dependency_type = 'goal_completion' AND (rg.is_completed = false OR rg.is_completed IS NULL))
                OR
                -- Зависимость от очков влияния не выполнена
                (gd.dependency_type = 'influence_threshold' AND (p.influence < gd.required_influence_points OR p.influence IS NULL))
            )
        ) THEN true
        
        -- Иначе цель скрыта
        ELSE false
    END AS is_visible,
    
    -- Определяем, заблокирована ли цель (видна, но не может быть выполнена)
    CASE 
        -- Если нет зависимостей - цель доступна
        WHEN NOT EXISTS (
            SELECT 1 FROM goal_dependencies gd WHERE gd.goal_id = g.id
        ) THEN false
        
        -- Проверяем, есть ли невыполненные и неразблокированные зависимости
        WHEN EXISTS (
            SELECT 1 FROM goal_dependencies gd
            LEFT JOIN goals rg ON gd.required_goal_id = rg.id
            LEFT JOIN players p ON gd.influence_player_id = p.id
            LEFT JOIN goal_dependency_unlocks gdu ON gd.id = gdu.dependency_id AND gd.goal_id = gdu.goal_id
            WHERE gd.goal_id = g.id 
            AND gdu.id IS NULL  -- Зависимость ещё не разблокирована
            AND (
                -- Зависимость от цели не выполнена
                (gd.dependency_type = 'goal_completion' AND (rg.is_completed = false OR rg.is_completed IS NULL))
                OR
                -- Зависимость от очков влияния не выполнена
                (gd.dependency_type = 'influence_threshold' AND (p.influence < gd.required_influence_points OR p.influence IS NULL))
            )
        ) THEN true
        
        -- Все условия выполнены или разблокированы - цель доступна
        ELSE false
    END AS is_locked
FROM goals g
WHERE g.goal_type = 'personal';

-- Представление для активных договоров
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

-- Счетчик выполненных задач по игрокам
CREATE OR REPLACE VIEW player_tasks_stats AS
SELECT 
    player_id,
    COUNT(*) AS total_tasks,
    COUNT(CASE WHEN is_completed = true THEN 1 END) AS completed_tasks,
    COUNT(CASE WHEN is_completed = false THEN 1 END) AS pending_tasks
FROM tasks
GROUP BY player_id;

-- Активные раунды гонки
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

-- Прогресс игрока в текущих гонках
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
-- КОММЕНТАРИИ К ТАБЛИЦАМ
-- ============================================

COMMENT ON TABLE factions IS 'Фракции в игре (дворец, мафия и т.д.)';
COMMENT ON TABLE players IS 'Игроки и их персонажи';
COMMENT ON TABLE items IS 'Предметы в игре';
COMMENT ON TABLE effects IS 'Эффекты, которые могут иметь предметы';
COMMENT ON TABLE abilities IS 'Уникальные способности игроков';
COMMENT ON TABLE goals IS 'Личные и фракционные цели';
COMMENT ON TABLE contracts IS 'Договоры между игроками';
COMMENT ON TABLE debt_receipts IS 'Долговые расписки';
COMMENT ON TABLE money_transactions IS 'История денежных транзакций для отслеживания админами';
COMMENT ON TABLE item_transactions IS 'История передачи предметов';
COMMENT ON TABLE influence_transactions IS 'История изменения очков влияния';
COMMENT ON TABLE tasks IS 'Задачи игроков (отличаются от целей)';
COMMENT ON TABLE goal_race_triggers IS 'Условия для запуска гонок целей';
COMMENT ON TABLE goal_race_rounds IS 'Раунды гонки целей';
COMMENT ON TABLE goal_race_predefined_goals IS 'Заранее созданные цели для будущих раундов гонки';
COMMENT ON TABLE goal_race_round_goals IS 'Назначение целей игрокам в рамках раунда';
COMMENT ON COLUMN goal_race_round_goals.is_accessible IS 'Становится false когда другой игрок завершает раунд';
COMMENT ON COLUMN goal_race_rounds.round_number IS 'Порядковый номер раунда в рамках одной гонки';
COMMENT ON COLUMN goal_race_rounds.status IS 'pending - создан, но не начат; active - текущий; completed - завершен; cancelled - отменен';

-- Таблица пользователей для авторизации
CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    username VARCHAR(255) NOT NULL UNIQUE,
    password VARCHAR(255) NOT NULL,
    player_id INTEGER REFERENCES players(id) ON DELETE SET NULL,
    is_admin BOOLEAN DEFAULT false
);

-- Индекс для быстрого поиска по username
CREATE INDEX idx_users_username ON users(username);

COMMENT ON TABLE users IS 'Пользователи системы для авторизации';
COMMENT ON COLUMN users.player_id IS 'Связь с персонажем игрока (может быть NULL для админов без персонажа)';
COMMENT ON COLUMN users.is_admin IS 'Флаг администратора системы';
-- Дополнительные индексы для новых таблиц зависимостей
CREATE INDEX idx_goal_dependencies_goal ON goal_dependencies(goal_id);
CREATE INDEX idx_goal_dependencies_required_goal ON goal_dependencies(required_goal_id);
CREATE INDEX idx_goal_dependencies_influence_player ON goal_dependencies(influence_player_id);
CREATE INDEX idx_goal_dependencies_type ON goal_dependencies(dependency_type);

CREATE INDEX idx_goal_dependency_unlocks_goal ON goal_dependency_unlocks(goal_id);
CREATE INDEX idx_goal_dependency_unlocks_dependency ON goal_dependency_unlocks(dependency_id);
CREATE INDEX idx_goal_dependency_unlocks_player ON goal_dependency_unlocks(player_id);

-- ============================================
-- ТРИГГЕРЫ ДЛЯ АВТОМАТИЧЕСКОЙ РАЗБЛОКИРОВКИ
-- ============================================

-- Функция для автоматической разблокировки зависимостей при изменении влияния
CREATE OR REPLACE FUNCTION unlock_goal_dependencies_on_influence_change()
RETURNS TRIGGER AS $$
BEGIN
    -- Когда у игрока меняется influence, проверяем все зависимости от его влияния
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
        AND gdu.id IS NULL  -- Ещё не разблокировано
    ON CONFLICT (goal_id, dependency_id) DO NOTHING;
    
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Триггер для автоматической разблокировки при изменении влияния
DROP TRIGGER IF EXISTS trigger_unlock_on_influence_change ON players;
CREATE TRIGGER trigger_unlock_on_influence_change
    AFTER UPDATE OF influence ON players
    FOR EACH ROW
    WHEN (OLD.influence IS DISTINCT FROM NEW.influence)
    EXECUTE FUNCTION unlock_goal_dependencies_on_influence_change();

-- Функция для автоматической разблокировки зависимостей при выполнении целей
CREATE OR REPLACE FUNCTION unlock_goal_dependencies_on_goal_completion()
RETURNS TRIGGER AS $$
BEGIN
    -- Когда цель выполняется, разблокируем все зависимости от неё
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
        AND gdu.id IS NULL  -- Ещё не разблокировано
    ON CONFLICT (goal_id, dependency_id) DO NOTHING;
    
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Триггер для автоматической разблокировки при выполнении целей
DROP TRIGGER IF EXISTS trigger_unlock_on_goal_completion ON goals;
CREATE TRIGGER trigger_unlock_on_goal_completion
    AFTER UPDATE OF is_completed ON goals
    FOR EACH ROW
    WHEN (OLD.is_completed IS DISTINCT FROM NEW.is_completed AND NEW.is_completed = true)
    EXECUTE FUNCTION unlock_goal_dependencies_on_goal_completion();

-- ============================================
-- ДОПОЛНИТЕЛЬНЫЕ КОММЕНТАРИИ К НОВЫМ ТАБЛИЦАМ
-- ============================================

COMMENT ON TABLE goal_dependencies IS 'Зависимости целей от выполнения других целей или от количества очков влияния других игроков. Поддерживает множественные зависимости.';
COMMENT ON TABLE goal_dependency_unlocks IS 'История разблокировок зависимостей - разблокированная зависимость остаётся разблокированной навсегда, даже если условие перестало выполняться';
COMMENT ON COLUMN goal_dependencies.dependency_type IS 'Тип зависимости: goal_completion (от выполнения цели) или influence_threshold (от порога влияния)';
COMMENT ON COLUMN goal_dependencies.is_visible_before_completion IS 'false = цель полностью скрыта до выполнения условия; true = цель видна, но отмечена как заблокированная';
COMMENT ON COLUMN goal_dependency_unlocks.unlocked_at IS 'Время когда зависимость была разблокирована. После этого цель остаётся доступной навсегда.';
COMMENT ON VIEW player_visible_goals IS 'Представление для определения видимости и доступности личных целей с учётом постоянных разблокировок';
COMMENT ON COLUMN player_visible_goals.is_locked IS 'true = цель видна, но заблокирована (есть невыполненные зависимости); false = цель доступна для выполнения';