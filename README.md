# new-year-role-game-backend

Это серверная часть для сайта новогодней ролевой игры

POST /api/auth/login - авторизация:
Запрос:
```
{
    "username": "your_usertname_here",
    "password": "your_password_here"
}
```
Ответ:
```
{
    "token": "your_jwt_token_here",
    "user": {
        "id": "user_id",
        "username": "username",
        "player_id": "player_id",
        "is_admin": "is_admin"
    }
}
```

GET /player/me - информация об игроке
Запрос:
```
Header "Authorization": "Bearer <jwt_token_here>"
{}
```
Ответ:
```
{
    "id": id,
    "name": "name",
    "role": "role",
    faction_id: faction_id,
    "can_change_faction": true/false,
    "description": "description",
    "info_about_players": [
        "Info 1",
        "info 2",
        <etc>
    ],
    "avatar": "<image in base64 here>"
}
```


/player/balance - баланс игрока
Запрос:
```
Header "Authorization": "Bearer <jwt_token_here>"
{}
```
Ответ:
```
{
    "money": money,
    "influence": influence
}
```




























