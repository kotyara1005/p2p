# p2p
При старте приложение находит другие инстансы этого приложения в локальной сети и устанавливает с ними p2p соединение. Раз в N секунд приложение отправляет всем сообщение со своим идентификатором и выводит сообщения, полученные от других приложений.

Запуск `docker-compose up --build --scale app=2`

Параметры приложения:
```
$ go run ./main.go -h
   -id string
     	Application id
   -interval int
     	Interval between sending messages (default 3)
   -ip string
     	Application ip
   -mask string
     	Network mask (default "29")
   -port string
     	Port for serving incoming notifications (default "53535")```