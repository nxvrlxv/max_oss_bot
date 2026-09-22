# Раздача webapp через Nginx (Ubuntu/Debian)

Команды выполняются на сервере из корня клонированного проекта.
Nginx отдаёт HTML напрямую: Node.js, бот и база для этого не нужны.

```bash
sudo apt update
sudo apt install -y nginx
sudo install -d -m 755 /var/www/max-webapp
sudo install -m 644 webapp/index.html /var/www/max-webapp/index.html
sudo install -m 644 deploy/nginx/max-webapp.conf /etc/nginx/sites-available/max-webapp
sudo nano /etc/nginx/sites-available/max-webapp
```

Замени `app.example.ru` в `server_name` своим доменом или IP сервера
для первоначальной HTTP-проверки. Не копируй весь репозиторий в папку сайта.

Включи конфигурацию (ссылка создаётся один раз):

```bash
sudo ln -s /etc/nginx/sites-available/max-webapp /etc/nginx/sites-enabled/max-webapp
sudo nginx -t
```

Только если проверка успешна:

```bash
sudo systemctl enable --now nginx
sudo systemctl reload nginx
curl -I -H 'Host: app.example.ru' http://127.0.0.1/
```

В проверке подставь такое же значение Host, как в `server_name`.
Открой `http://ТВОЙ-ДОМЕН/` (или IP, если его указал в конфигурации).
Ожидается «Привет, мир!». Для внешнего доступа разреши входящий TCP-порт 80
в firewall сервера и панели хостинга. Другие сайты отключать не требуется.

## HTTPS для MAX

HTTP-конфигурация выше предназначена для первоначальной проверки.
В MAX нужно указывать HTTPS-ссылку.

1. Направь DNS-запись A домена на IPv4 сервера. Если есть AAAA, IPv6 тоже
   должен вести на этот сервер и обслуживаться Nginx.
2. Укажи домен в `server_name`, выполни `sudo nginx -t` и перезагрузи Nginx.
3. Открой входящие TCP-порты 80 и 443.
4. Выпусти сертификат для домена, например через Certbot:

```bash
sudo apt install -y certbot python3-certbot-nginx
sudo certbot --nginx -d app.example.ru --redirect
sudo nginx -t
sudo certbot renew --dry-run
```

Замени `app.example.ru` своим доменом. Certbot добавит HTTPS в установленную
конфигурацию и перенаправление с HTTP. После этого не перезаписывай её исходным
HTTP-шаблоном из репозитория.

Проверь `https://ТВОЙ-ДОМЕН/` и укажи этот URL в настройках мини-приложения MAX.
Сертификаты доверия к API MAX не подходят вместо сертификата твоего домена.

## HTTPS с уже выпущенным сертификатом

Используй `max-webapp-https.conf` вместо HTTP-шаблона, если у тебя уже есть
сертификат на собственный домен и соответствующий приватный ключ.
Корневой и промежуточный сертификаты Минцифры для доверия к API MAX
не являются сертификатом твоего сайта и для этой настройки не подходят.

Нужны PEM-файлы:

- `fullchain.pem`: сертификат сайта первым, затем промежуточные сертификаты;
- `privkey.pem`: соответствующий приватный ключ (хранится только на сервере).

Установи их до включения HTTPS-конфига:

```bash
sudo install -d -m 700 /etc/nginx/ssl/max-webapp
sudo install -m 644 /путь/к/fullchain.pem /etc/nginx/ssl/max-webapp/fullchain.pem
sudo install -m 600 /путь/к/privkey.pem /etc/nginx/ssl/max-webapp/privkey.pem
sudo install -m 644 deploy/nginx/max-webapp-https.conf /etc/nginx/sites-available/max-webapp
sudo nano /etc/nginx/sites-available/max-webapp
```

Замени все три вхождения `app.example.ru` своим доменом. Если сайт ещё не включён,
создай ссылку в `sites-enabled` по инструкции выше. Не включай оба шаблона
одновременно для одного домена. Существующая ссылка на `sites-available/max-webapp`
продолжит работать после замены содержимого файла.

```bash
sudo nginx -t
```

Если проверка успешна:

```bash
sudo systemctl reload nginx
curl -I https://app.example.ru/
```

Подставь свой домен. Ожидается HTTP 200; HTTP-адрес перенаправляет на HTTPS.
Домен должен вести на сервер, порты 80 и 443 — быть доступны.
Конфиг слушает IPv4: если публикуешь AAAA-запись, добавь соответствующие
`listen [::]:80;` и `listen [::]:443 ssl;` в блоки server.

После обновления сертификата или ключа повтори установку файлов, проверку
`nginx -t` и reload. Сам шаблон не обеспечивает автоматическое продление.
Для сертификатов под управлением Certbot используй предыдущий раздел,
где пути и продление настраиваются Certbot.

## Обновление страницы

После обновления исходников повтори:

```bash
sudo install -m 644 webapp/index.html /var/www/max-webapp/index.html
```

Для изменения HTML перезапуск Nginx не нужен.

Документация: [Nginx](https://nginx.org/en/docs/http/ngx_http_core_module.html#try_files),
[Certbot](https://certbot.eff.org/instructions),
[подключение MAX](https://dev.max.ru/help/miniapps).
