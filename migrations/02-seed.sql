-- migrations/02-seed.sql
-- Mock данные для тестирования ролевой игры

-- ============================================
-- ФРАКЦИИ
-- ============================================

INSERT INTO factions (name, description, faction_influence, is_composition_visible_to_all) VALUES
('Дворец', 'Королевская фракция, представители высшей знати', 50, true),
('Мафия', 'Теневая организация, контролирующая преступный мир', 30, false),
('Торговая гильдия', 'Объединение купцов и ремесленников', 40, true),
('Церковь', 'Религиозная организация с большим влиянием', 35, true);

-- ============================================
-- ИГРОКИ
-- ============================================

INSERT INTO players (character_name, password, character_story, role, money, influence, faction_id, can_change_faction, avatar) VALUES
-- Дворец
('Король Артур', 'password123', 'Мудрый правитель королевства', 'Правитель', 1000, 100, 1, false, NULL),
('Принцесса Элизабет', 'password123', 'Наследница престола, интересуется магией', 'Принцесса', 800, 80, 1, false, NULL),
('Советник Мерлин', 'password123', 'Главный советник короля, обладает тайными знаниями', 'Советник', 600, 70, 1, false, NULL),

-- Мафия
('Дон Корлеоне', 'password123', 'Глава мафиозной семьи', 'Босс мафии', 1500, 90, 2, false, NULL),
('Консильери Том', 'password123', 'Правая рука дона, юрист', 'Консильери', 700, 60, 2, false, NULL),
('Киллер Винченцо', 'password123', 'Исполнитель особых поручений', 'Киллер', 500, 50, 2, false, NULL),

-- Торговая гильдия
('Купец Марко', 'password123', 'Богатый торговец экзотическими товарами', 'Купец', 2000, 75, 3, false, NULL),
('Ювелир Сара', 'password123', 'Мастер ювелирного дела', 'Ювелир', 900, 55, 3, false, NULL),

-- Церковь
('Архиепископ Бенедикт', 'password123', 'Глава церкви', 'Архиепископ', 800, 85, 4, false, NULL),
('Инквизитор Даниэль', 'password123', 'Борец с ересью', 'Инквизитор', 400, 65, 4, false, NULL),

-- Нейтральные
('Доктор Ватсон', 'password123', 'Лекарь, помогающий всем без разбора', 'Врач', 500, 40, NULL, true, NULL),
('Шпион Джеймс', 'password123', 'Тайный агент, собирающий информацию', 'Шпион', 600, 45, NULL, true, NULL),
('Кондитер Мари', 'password123', 'Владелица лучшей кондитерской в городе', 'Кондитер', 400, 30, NULL, false, NULL);

-- Обновляем лидеров фракций
UPDATE factions SET leader_player_id = 1 WHERE id = 1; -- Король
UPDATE factions SET leader_player_id = 4 WHERE id = 2; -- Дон
UPDATE factions SET leader_player_id = 7 WHERE id = 3; -- Купец
UPDATE factions SET leader_player_id = 9 WHERE id = 4; -- Архиепископ

-- ============================================
-- ПОЛЬЗОВАТЕЛИ (для авторизации)
-- ============================================

INSERT INTO users (username, password, player_id, is_admin) VALUES
('admin', 'admin123', NULL, true),
('arthur', 'password123', 1, false),
('elizabeth', 'password123', 2, false),
('merlin', 'password123', 3, false),
('don', 'password123', 4, false),
('tom', 'password123', 5, false),
('vinny', 'password123', 6, false),
('marco', 'password123', 7, false),
('sarah', 'password123', 8, false),
('benedict', 'password123', 9, false),
('daniel', 'password123', 10, false),
('watson', 'password123', 11, false),
('james', 'password123', 12, false),
('marie', 'password123', 13, false);

-- ============================================
-- ИНФОРМАЦИЯ О ДРУГИХ ИГРОКАХ
-- ============================================

INSERT INTO info_about_other_players (player_id, description) VALUES
(1, 'Король известен своей справедливостью, но слухи говорят о тайной болезни'),
(4, 'Дон контролирует половину городской торговли через подставных лиц'),
(12, 'Шпион работает сразу на несколько сторон, его истинная лояльность неизвестна');

-- ============================================
-- ПРЕДМЕТЫ
-- ============================================

INSERT INTO items (name, description) VALUES
('Королевская печать', 'Позволяет издавать указы от имени короля'),
('Секретные документы', 'Компромат на влиятельных персон'),
('Золотой слиток', 'Чистое золото высшей пробы'),
('Лечебное зелье', 'Восстанавливает здоровье'),
('Ключ от сокровищницы', 'Открывает доступ к королевской казне'),
('Контрабанда', 'Нелегальный товар высокой ценности'),
('Древний артефакт', 'Магический предмет неизвестного происхождения'),
('Шпионское оборудование', 'Инструменты для слежки'),
('Ювелирные изделия', 'Дорогие украшения'),
('Святые реликвии', 'Предметы церковного культа');

-- ============================================
-- ЭФФЕКТЫ
-- ============================================

INSERT INTO effects (description, effect_type, generated_resource, operation, value, spawned_item_id, period_seconds) VALUES
-- Генерация денег
('Приносит 100 золотых каждый час', 'generate_money', 'money', 'add', 100, NULL, 3600),
('Приносит 50 золотых каждые 30 минут', 'generate_money', 'money', 'add', 50, NULL, 1800),
('Приносит 200 золотых каждые 2 часа', 'generate_money', 'money', 'add', 200, NULL, 7200),

-- Генерация влияния
('Приносит 5 очков влияния каждый час', 'generate_influence', 'influence', 'add', 5, NULL, 3600),
('Приносит 10 очков влияния каждые 2 часа', 'generate_influence', 'influence', 'add', 10, NULL, 7200),

-- Спавн предметов
('Генерирует золотой слиток каждые 3 часа', 'spawn_item', NULL, 'add', NULL, 3, 10800),
('Генерирует лечебное зелье каждый час', 'spawn_item', NULL, 'add', NULL, 4, 3600);

-- ============================================
-- СВЯЗЬ ПРЕДМЕТОВ И ЭФФЕКТОВ
-- ============================================

INSERT INTO item_effects (item_id, effect_id) VALUES
(1, 1), -- Королевская печать приносит деньги
(1, 4), -- Королевская печать приносит влияние
(5, 3), -- Ключ от сокровищницы приносит много денег
(6, 2), -- Контрабанда приносит деньги
(7, 5), -- Древний артефакт приносит влияние
(9, 6); -- Ювелирные изделия генерируют золото

-- ============================================
-- ИНВЕНТАРЬ ИГРОКОВ
-- ============================================

INSERT INTO player_items (player_id, item_id) VALUES
(1, 1), -- Король имеет печать
(1, 5), -- Король имеет ключ от сокровищницы
(4, 2), -- Дон имеет секретные документы
(4, 6), -- Дон имеет контрабанду
(7, 9), -- Купец имеет ювелирные изделия
(11, 4), -- Доктор имеет лечебное зелье
(12, 8); -- Шпион имеет шпионское оборудование

-- ============================================
-- ИНИЦИАЛИЗАЦИЯ ВЫПОЛНЕНИЯ ЭФФЕКТОВ
-- ============================================

INSERT INTO item_effect_executions (player_id, item_id, effect_id, last_executed_at) VALUES
(1, 1, 1, NOW() - INTERVAL '30 minutes'),
(1, 1, 4, NOW() - INTERVAL '45 minutes'),
(4, 6, 2, NOW() - INTERVAL '20 minutes');

-- ============================================
-- УНИКАЛЬНЫЕ СПОСОБНОСТИ
-- ============================================

INSERT INTO abilities (player_id, name, description, ability_type, cooldown_minutes, start_delay_minutes, required_influence_points, is_unlocked, influence_points_to_add, influence_points_to_remove, influence_points_to_self) VALUES
-- Способность короля - раскрытие информации
(1, 'Королевская разведка', 'Раскрыть один факт о любом персонаже', 'reveal_info', 40, 0, NULL, true, NULL, NULL, NULL),

-- Способность дона - раскрытие информации
(4, 'Мафиозная сеть', 'Узнать секрет любого персонажа', 'reveal_info', 60, 30, NULL, true, NULL, NULL, NULL),

-- Способность советника - добавление влияния
(3, 'Мудрый совет', 'Дать мудрый совет, повышающий влияние другого игрока', 'add_influence', NULL, 0, 50, true, 15, NULL, NULL),

-- Способность инквизитора - перенос влияния (одноразовая)
(10, 'Инквизиция', 'Обвинить в ереси и забрать влияние', 'transfer_influence', NULL, 0, NULL, true, NULL, 20, 15),

-- Способность шпиона - раскрытие информации
(12, 'Шпионаж', 'Выведать секрет', 'reveal_info', 45, 0, NULL, true, NULL, NULL, NULL),

-- Заблокированная способность принцессы
(2, 'Магия света', 'Даровать благословение', 'add_influence', NULL, 20, 70, false, 20, NULL, NULL);

-- ============================================
-- ЦЕЛИ
-- ============================================

-- Личные цели
INSERT INTO goals (title, description, goal_type, influence_points_reward, player_id, faction_id, is_completed) VALUES
-- Цели короля (ID 1-5)
('Укрепить королевство', 'Заключить 3 выгодных договора', 'personal', 30, 1, NULL, false),
('Разобраться с коррупцией', 'Найти и наказать коррумпированных чиновников', 'personal', 25, 1, NULL, false),
('Провести реформы', 'Модернизировать систему управления', 'personal', 35, 1, NULL, false),
('Нейтрализовать угрозу', 'Остановить растущее влияние мафии', 'personal', 50, 1, NULL, false), -- Будет скрыта до набора влияния Доном
('Объединить королевство', 'Достичь мира со всеми фракциями', 'personal', 100, 1, NULL, false), -- Финальная цель короля

-- Цели принцессы (ID 6-8)
('Изучить древнюю магию', 'Найти и прочитать запретные книги', 'personal', 20, 2, NULL, false),
('Получить благословение', 'Заручиться поддержкой церкви', 'personal', 25, 2, NULL, false),
('Раскрыть заговор', 'Узнать кто плетет интриги против трона', 'personal', 40, 2, NULL, false), -- Зависит от выполнения предыдущей

-- Цели советника (ID 9-10)
('Разгадать пророчество', 'Интерпретировать древнее предсказание', 'personal', 30, 3, NULL, false),
('Защитить короля', 'Обеспечить безопасность правителя', 'personal', 35, 3, NULL, false),

-- Цели дона (ID 11-14)
('Расширить влияние', 'Подчинить себе новые территории', 'personal', 40, 4, NULL, false),
('Устранить конкурента', 'Избавиться от мешающего делу человека', 'personal', 35, 4, NULL, false),
('Заключить союз', 'Договориться с торговой гильдией', 'personal', 30, 4, NULL, false),
('Захватить власть', 'Стать теневым правителем города', 'personal', 80, 4, NULL, false), -- Требует высокого влияния и выполнения целей

-- Цели консильери (ID 15-16)
('Собрать информацию', 'Разведать планы конкурентов', 'personal', 20, 5, NULL, false),
('Укрепить позиции семьи', 'Улучшить репутацию в обществе', 'personal', 25, 5, NULL, false),

-- Цели киллера (ID 17-18)
('Выполнить контракт', 'Устранить указанную цель', 'personal', 30, 6, NULL, false),
('Стать легендой', 'Прославиться как лучший исполнитель', 'personal', 45, 6, NULL, false),

-- Цели купца (ID 19-21)
('Монополизировать рынок', 'Стать единственным поставщиком редких товаров', 'personal', 40, 7, NULL, false),
('Открыть новый филиал', 'Расширить торговую сеть', 'personal', 25, 7, NULL, false),
('Разбогатеть', 'Накопить 5000 золотых', 'personal', 35, 7, NULL, false),

-- Цели ювелира (ID 22-23)
('Создать шедевр', 'Изготовить украшение для королевы', 'personal', 30, 8, NULL, false),
('Найти редкий камень', 'Раздобыть легендарный алмаз', 'personal', 40, 8, NULL, false),

-- Цели архиепископа (ID 24-25)
('Провести великую мессу', 'Собрать всех верующих на службу', 'personal', 30, 9, NULL, false),
('Искоренить ересь', 'Очистить город от темных культов', 'personal', 35, 9, NULL, false),

-- Цели инквизитора (ID 26-27)
('Допросить еретиков', 'Выявить отступников от веры', 'personal', 25, 10, NULL, false),
('Укрепить веру', 'Обратить неверующих в истинную религию', 'personal', 30, 10, NULL, false),

-- Цели доктора (ID 28-30)
('Найти лекарство', 'Разработать средство от новой болезни', 'personal', 35, 11, NULL, false),
('Спасти важную персону', 'Вылечить влиятельного пациента', 'personal', 30, 11, NULL, false),
('Открыть больницу', 'Создать лечебницу для бедных', 'personal', 40, 11, NULL, false),

-- Цели шпиона (ID 31-35)
('Собрать улики', 'Найти компрометирующие материалы', 'personal', 20, 12, NULL, false),
('Внедриться в организацию', 'Стать доверенным лицом во фракции', 'personal', 35, 12, NULL, false), -- Зависит от предыдущей
('Раскрыть заговор', 'Узнать о планах переворота', 'personal', 40, 12, NULL, false), -- Зависит от влияния дона
('Стать двойным агентом', 'Работать на две стороны одновременно', 'personal', 50, 12, NULL, false), -- Комбинированная зависимость
('Исчезнуть бесследно', 'Получить новую личность и скрыться', 'personal', 60, 12, NULL, false), -- Финальная цель

-- Цели кондитера (ID 36-37)
('Испечь королевский торт', 'Создать десерт для дворцового банкета', 'personal', 25, 13, NULL, false),
('Открыть вторую кондитерскую', 'Расширить бизнес', 'personal', 30, 13, NULL, false);

-- Командные цели фракций (ID 38-45)
INSERT INTO goals (title, description, goal_type, influence_points_reward, player_id, faction_id, is_completed) VALUES
-- Цели Дворца
('Укрепить оборону', 'Нанять дополнительную стражу', 'faction', 40, NULL, 1, false),
('Провести бал', 'Организовать великосветский прием', 'faction', 30, NULL, 1, false),

-- Цели Мафии
('Расширить территорию', 'Захватить новые районы города', 'faction', 45, NULL, 2, false),
('Устранить свидетеля', 'Избавиться от опасного информатора', 'faction', 35, NULL, 2, false),

-- Цели Торговой гильдии
('Открыть торговый путь', 'Наладить торговлю с соседним королевством', 'faction', 40, NULL, 3, false),
('Провести ярмарку', 'Организовать грандиозную торговую ярмарку', 'faction', 30, NULL, 3, false),

-- Цели Церкви
('Построить храм', 'Возвести новую церковь', 'faction', 50, NULL, 4, false),
('Провести крестовый поход', 'Организовать священную войну против неверных', 'faction', 60, NULL, 4, false);

-- ============================================
-- ЗАВИСИМОСТИ ЦЕЛЕЙ (НОВОЕ!)
-- ============================================

-- Зависимости целей короля
-- Цель "Нейтрализовать угрозу" (ID 4) появляется когда Дон набирает опасное влияние
INSERT INTO goal_dependencies (goal_id, dependency_type, influence_player_id, required_influence_points, is_visible_before_completion) VALUES
(4, 'influence_threshold', 4, 100, false); -- Скрыта до тех пор, пока Дон не наберёт 100 влияния

-- Финальная цель короля "Объединить королевство" (ID 5) требует выполнения нескольких предыдущих целей
INSERT INTO goal_dependencies (goal_id, dependency_type, required_goal_id, is_visible_before_completion) VALUES
(5, 'goal_completion', 1, true),  -- Нужно укрепить королевство
(5, 'goal_completion', 3, true);  -- Нужно провести реформы

-- Зависимости целей принцессы
-- Цель "Раскрыть заговор" (ID 8) зависит от выполнения предыдущих
INSERT INTO goal_dependencies (goal_id, dependency_type, required_goal_id, is_visible_before_completion) VALUES
(8, 'goal_completion', 6, true),  -- Нужно изучить древнюю магию
(8, 'goal_completion', 7, false); -- Нужно получить благословение (эта часть скрыта)

-- Зависимости целей дона
-- Финальная цель "Захватить власть" (ID 14) требует высокого влияния и выполнения целей
INSERT INTO goal_dependencies (goal_id, dependency_type, required_goal_id, is_visible_before_completion) VALUES
(14, 'goal_completion', 11, true), -- Расширить влияние
(14, 'goal_completion', 13, true); -- Заключить союз

-- И требует чтобы Король потерял влияние (упал ниже порога)
-- На самом деле, давайте сделаем по-другому: требует чтобы сам Дон набрал много влияния
INSERT INTO goal_dependencies (goal_id, dependency_type, influence_player_id, required_influence_points, is_visible_before_completion) VALUES
(14, 'influence_threshold', 4, 120, true); -- Дон должен сам набрать 120 влияния

-- Зависимости целей шпиона (демонстрируют разные комбинации)
-- "Внедриться в организацию" (ID 32) требует выполнения "Собрать улики" (ID 31)
INSERT INTO goal_dependencies (goal_id, dependency_type, required_goal_id, is_visible_before_completion) VALUES
(32, 'goal_completion', 31, true);

-- "Раскрыть заговор" (ID 33) требует чтобы Дон набрал опасное влияние
INSERT INTO goal_dependencies (goal_id, dependency_type, influence_player_id, required_influence_points, is_visible_before_completion) VALUES
(33, 'influence_threshold', 4, 100, true); -- Видна, но заблокирована

-- "Стать двойным агентом" (ID 34) - комбинированная зависимость
INSERT INTO goal_dependencies (goal_id, dependency_type, required_goal_id, is_visible_before_completion) VALUES
(34, 'goal_completion', 32, false); -- Нужно внедриться (скрыто)

INSERT INTO goal_dependencies (goal_id, dependency_type, influence_player_id, required_influence_points, is_visible_before_completion) VALUES
(34, 'influence_threshold', 4, 110, false), -- Дон должен набрать 110 влияния
(34, 'influence_threshold', 1, 90, false);  -- Король должен иметь 90 влияния

-- "Исчезнуть бесследно" (ID 35) - финальная цель шпиона
INSERT INTO goal_dependencies (goal_id, dependency_type, required_goal_id, is_visible_before_completion) VALUES
(35, 'goal_completion', 34, false);

-- Зависимости целей купца
-- "Монополизировать рынок" (ID 19) требует высокого влияния самого купца и устранения конкурентов
INSERT INTO goal_dependencies (goal_id, dependency_type, influence_player_id, required_influence_points, is_visible_before_completion) VALUES
(19, 'influence_threshold', 7, 100, true), -- Купец должен набрать 100 влияния
(19, 'influence_threshold', 8, 70, true);  -- Ювелир должна набрать 70 (партнерство)

-- Зависимость цели доктора
-- "Открыть больницу" (ID 30) требует выполнения предыдущих целей и богатства
INSERT INTO goal_dependencies (goal_id, dependency_type, required_goal_id, is_visible_before_completion) VALUES
(30, 'goal_completion', 28, true), -- Найти лекарство
(30, 'goal_completion', 29, true); -- Спасти важную персону

-- ============================================
-- РАЗБЛОКИРОВКИ ЗАВИСИМОСТЕЙ (демонстрация)
-- ============================================

-- Симулируем что некоторые условия уже были выполнены в прошлом
-- Например, Дон уже набирал 100 влияния, но потом потерял часть из-за штрафа
-- Это демонстрирует постоянную разблокировку

-- Разблокируем зависимость цели короля "Нейтрализовать угрозу" от влияния Дона
INSERT INTO goal_dependency_unlocks (goal_id, dependency_id, player_id, unlocked_at) VALUES
(4, 
 (SELECT id FROM goal_dependencies WHERE goal_id = 4 AND influence_player_id = 4),
 1, 
 NOW() - INTERVAL '1 day');

-- Разблокируем одну из зависимостей шпиона (цель "Раскрыть заговор")
INSERT INTO goal_dependency_unlocks (goal_id, dependency_id, player_id, unlocked_at) VALUES
(33,
 (SELECT id FROM goal_dependencies WHERE goal_id = 33 AND influence_player_id = 4),
 12,
 NOW() - INTERVAL '6 hours');

-- ============================================
-- ЗАДАЧИ ИГРОКОВ
-- ============================================

INSERT INTO tasks (player_id, title, description, is_completed) VALUES
-- Задачи короля
(1, 'Провести королевский совет', 'Собрать всех советников для обсуждения', true),
(1, 'Проверить казну', 'Провести ревизию королевской сокровищницы', false),
(1, 'Принять послов', 'Встретиться с представителями соседних королевств', false),

-- Задачи дона
(4, 'Собрать дань', 'Получить плату за защиту от торговцев', true),
(4, 'Организовать встречу', 'Провести тайную встречу с союзниками', true),
(4, 'Заказать спецоперацию', 'Поручить выполнение деликатного задания', false),

-- Задачи купца
(7, 'Закупить товары', 'Приобрести партию экзотических специй', true),
(7, 'Найти новых клиентов', 'Расширить клиентскую базу', false),

-- Задачи доктора
(11, 'Вылечить больного', 'Помочь пациенту с редкой болезнью', true),
(11, 'Собрать травы', 'Найти редкие лечебные растения', false),

-- Задачи шпиона
(12, 'Собрать информацию', 'Узнать о планах мафии', true),
(12, 'Передать отчет', 'Доложить собранные данные заказчику', true),
(12, 'Проникнуть во дворец', 'Получить доступ к секретным документам', false);

-- ============================================
-- ТРИГГЕРЫ ГОНКИ ЦЕЛЕЙ
-- ============================================

INSERT INTO goal_race_triggers (name, description, required_tasks_count, is_active) VALUES
('Гонка за власть', 'При выполнении 3 задач начинается гонка за специальные цели', 3, true);

-- ============================================
-- ПРЕДОПРЕДЕЛЕННЫЕ ЦЕЛИ ДЛЯ ГОНКИ
-- ============================================

INSERT INTO goal_race_predefined_goals (trigger_id, round_number, player_id, title, description, influence_points_reward) VALUES
-- Раунд 1
(1, 1, 1, 'Издать указ', 'Использовать королевскую печать для издания важного указа', 25),
(1, 1, 4, 'Устранить свидетеля', 'Избавиться от неудобного свидетеля', 25),
(1, 1, 7, 'Заключить сделку века', 'Провести самую крупную торговую операцию', 25),
(1, 1, 12, 'Украсть секрет', 'Получить важную информацию для заказчика', 25),

-- Раунд 2
(1, 2, 1, 'Созвать парламент', 'Собрать представителей всех фракций', 30),
(1, 2, 4, 'Организовать переворот', 'Подготовить захват власти', 30),
(1, 2, 7, 'Открыть новый торговый путь', 'Наладить торговлю с дальними землями', 30),
(1, 2, 12, 'Стать двойным агентом', 'Работать на две стороны одновременно', 30);

-- ============================================
-- УЧАСТНИКИ ТРИГГЕРА
-- ============================================

INSERT INTO goal_race_trigger_participants (trigger_id, player_id) VALUES
(1, 1),  -- Король
(1, 4),  -- Дон
(1, 7),  -- Купец
(1, 12); -- Шпион

-- ============================================
-- ДОГОВОРЫ
-- ============================================

INSERT INTO contracts (contract_type, customer_player_id, executor_player_id, customer_faction_id, status, duration_seconds, money_reward_customer, money_reward_executor, signed_at, expires_at) VALUES
-- Активный договор
('type1', 1, 7, 1, 'signed', 86400, 0, 300, NOW(), NOW() + INTERVAL '1 day'),

-- Ожидающий подписания
('type2', 4, 12, 2, 'pending', 43200, 0, 500, NULL, NULL),

-- Завершенный договор
('type1', 7, 11, 3, 'completed', 3600, 0, 200, NOW() - INTERVAL '2 hours', NOW() - INTERVAL '1 hour');

-- ============================================
-- НАСТРОЙКИ НАГРАД ДЛЯ ДОГОВОРОВ ТИПА 1
-- ============================================

INSERT INTO contract_type1_settings (faction_id, customer_item_reward_id) VALUES
(1, 1),  -- Дворец получает королевскую печать
(2, 2),  -- Мафия получает секретные документы
(3, 9),  -- Гильдия получает ювелирные изделия
(4, 10); -- Церковь получает святые реликвии

-- ============================================
-- НАСТРОЙКИ ШТРАФОВ ЗА НАРУШЕНИЕ ДОГОВОРОВ
-- ============================================

INSERT INTO contract_penalty_settings (money_penalty, influence_penalty) VALUES
(500, 20);

-- ============================================
-- ДОЛГОВЫЕ РАСПИСКИ
-- ============================================

INSERT INTO debt_receipts (lender_player_id, borrower_player_id, loan_amount, return_amount, return_deadline, is_returned) VALUES
-- Активная расписка
(7, 11, 200, 250, NOW() + INTERVAL '2 days', false),

-- Просроченная расписка
(1, 13, 150, 200, NOW() - INTERVAL '1 day', false),

-- Возвращенная расписка
(4, 5, 300, 350, NOW() + INTERVAL '1 day', true);

-- ============================================
-- НАСТРОЙКИ ШТРАФОВ ЗА ДОЛГИ
-- ============================================

INSERT INTO debt_penalty_settings (penalty_influence_points) VALUES
(15);

-- ============================================
-- ТРАНЗАКЦИИ
-- ============================================

-- Денежные транзакции
INSERT INTO money_transactions (from_player_id, to_player_id, amount, transaction_type, description) VALUES
(1, 7, 300, 'contract', 'Выплата по договору'),
(7, 11, 200, 'debt', 'Выдача займа'),
(4, 5, 100, 'transfer', 'Оплата услуг');

-- Транзакции предметов
INSERT INTO item_transactions (from_player_id, to_player_id, item_id, transaction_type, description) VALUES
(NULL, 1, 1, 'spawned', 'Создание королевской печати'),
(7, 4, 9, 'transfer', 'Передача ювелирных изделий');

-- Транзакции влияния
INSERT INTO influence_transactions (player_id, amount, transaction_type, description) VALUES
(1, 30, 'goal', 'Выполнение цели "Укрепить королевство"'),
(4, 40, 'goal', 'Выполнение цели "Расширить влияние"'),
(12, -10, 'penalty', 'Штраф за нарушение договора');

-- ============================================
-- ИГРОВЫЕ НАСТРОЙКИ
-- ============================================

INSERT INTO game_settings (setting_key, setting_value, description) VALUES
('max_faction_changes', '1', 'Максимальное количество смен фракции для игрока'),
('contract_penalty_enabled', 'true', 'Включены ли штрафы за нарушение договоров'),
('debt_penalty_enabled', 'true', 'Включены ли штрафы за просрочку долгов'),
('goal_race_enabled', 'true', 'Включена ли система гонки целей');

-- ============================================
-- ВРЕМЕННАЯ МЕТКА ИГРЫ
-- ============================================

INSERT INTO game_timeline (game_started_at) VALUES
(NOW() - INTERVAL '2 hours');

-- ============================================
-- ИСТОРИЯ ИСПОЛЬЗОВАНИЯ СПОСОБНОСТЕЙ
-- ============================================

INSERT INTO ability_usage (player_id, ability_id, target_player_id, info_category, used_at) VALUES
(1, 1, 4, 'faction', NOW() - INTERVAL '1 hour'),
(12, 5, 1, 'goal', NOW() - INTERVAL '30 minutes');

-- ============================================
-- РАСКРЫТАЯ ИНФОРМАЦИЯ
-- ============================================

INSERT INTO revealed_info (revealer_player_id, target_player_id, info_type, revealed_data, ability_usage_id) VALUES
(1, 4, 'faction', '{"faction_name": "Мафия", "faction_id": 2}', 1),
(12, 1, 'goal', '{"goal_title": "Укрепить королевство"}', 2);

-- ============================================
-- ЗАВЕРШЕНИЕ
-- ============================================

-- Вывод статистики
DO $$
BEGIN
    RAISE NOTICE '==============================================';
    RAISE NOTICE 'Seed данные успешно загружены!';
    RAISE NOTICE '==============================================';
    RAISE NOTICE 'Фракций: %', (SELECT COUNT(*) FROM factions);
    RAISE NOTICE 'Игроков: %', (SELECT COUNT(*) FROM players);
    RAISE NOTICE 'Пользователей: %', (SELECT COUNT(*) FROM users);
    RAISE NOTICE 'Предметов: %', (SELECT COUNT(*) FROM items);
    RAISE NOTICE 'Личных целей: %', (SELECT COUNT(*) FROM goals WHERE goal_type = 'personal');
    RAISE NOTICE 'Командных целей: %', (SELECT COUNT(*) FROM goals WHERE goal_type = 'faction');
    RAISE NOTICE 'Зависимостей целей: %', (SELECT COUNT(*) FROM goal_dependencies);
    RAISE NOTICE '  - От выполнения целей: %', (SELECT COUNT(*) FROM goal_dependencies WHERE dependency_type = 'goal_completion');
    RAISE NOTICE '  - От влияния игроков: %', (SELECT COUNT(*) FROM goal_dependencies WHERE dependency_type = 'influence_threshold');
    RAISE NOTICE 'Разблокированных зависимостей: %', (SELECT COUNT(*) FROM goal_dependency_unlocks);
    RAISE NOTICE 'Договоров: %', (SELECT COUNT(*) FROM contracts);
    RAISE NOTICE 'Долговых расписок: %', (SELECT COUNT(*) FROM debt_receipts);
    RAISE NOTICE '==============================================';
    RAISE NOTICE 'ПРИМЕРЫ ЗАВИСИМОСТЕЙ ЦЕЛЕЙ:';
    RAISE NOTICE '- Король: цель "Нейтрализовать угрозу" разблокирована (Дон набирал 100 влияния)';
    RAISE NOTICE '- Шпион: цель "Раскрыть заговор" разблокирована (Дон набирал 100 влияния)';
    RAISE NOTICE '- Шпион: цель "Стать двойным агентом" требует 3 условий (цель + 2 игрока с влиянием)';
    RAISE NOTICE '- Дон: цель "Захватить власть" требует 120 влияния + 2 выполненные цели';
    RAISE NOTICE '==============================================';
END $$;