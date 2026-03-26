# NOTES (Docs vs Requirements)

1. MAX production polling:
   - В требованиях допускается webhook/subscriptions и обсуждается long polling.
   - По официальным рекомендациям MAX long polling предназначен для тестов; в production выбран webhook-only.
2. Telegram read-all:
   - Даже при членстве бота в группе получение всех сообщений может быть ограничено privacy mode.
   - Поэтому readiness не бинарный, а `READY | LIMITED | BLOCKED`.
3. MAX webhook secret:
   - Проверка выполняется по заголовку `X-Max-Bot-Api-Secret`.
4. Telegram webhook secret:
   - Проверка по `X-Telegram-Bot-Api-Secret-Token`.
