package main

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
	"net/http"
	"text/template"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var users map[string]User

func init() {
	users = make(map[string]User)
}

var upgrader = websocket.Upgrader{} // use default option

var conns []*websocket.Conn

func echo(ctx *gin.Context) {
	w, r := ctx.Writer, ctx.Request
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Error().Err(err).Msg("could not upgrade connection")
		return
	}
	conns = append(conns, c)
	log.Info()
	defer c.Close()
	for {
		mt, message, err := c.ReadMessage()
		if err != nil {
			break
		}
		log.Printf("recv:%s", message)
		for _, client := range conns {
			err = client.WriteMessage(mt, message)
			if err != nil {
				log.Error().Err(err).Msg("could not send message")
				break
			}
		}
	}
}

type AuthRequest struct {
	Name string `json:"name"`
}

func auth(c *gin.Context) {
	var r AuthRequest
	err := c.BindJSON(&r)
	if err != nil {
		log.Error().Err(err).Msg("could not parse request")
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	id := uuid.New()
	msg, _ := id.MarshalText()

	users[string(msg)] = User{
		Name:      r.Name,
		SessionID: string(msg),
	}
	log.Info().Interface("users", users).Msg("added new user")

	c.JSON(http.StatusOK, gin.H{
		"session_id": string(msg),
	})
}

func home(c *gin.Context) {
	homeTemplate.Execute(c.Writer, "ws://"+c.Request.Host+"/echo")
}

func ExampleClient() {
	var ctx = context.Background()

	rdb := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "", // no password set
		DB:       0,  // use default DB
	})

	err := rdb.Set(ctx, "key", "value", 0).Err()
	if err != nil {
		panic(err)
	}

	val, err := rdb.Get(ctx, "key").Result()
	if err != nil {
		panic(err)
	}
	fmt.Println("key", val)
}

func main() {

	ExampleClient()

	r := gin.Default()
	r.GET("/echo", echo)
	r.GET("/", home)
	r.POST("/api/auth", auth)

	err := r.Run("0.0.0.0:8080")
	if err != nil {
		log.Fatal().Err(err).Msg("could not start server")
	}
}

var homeTemplate = template.Must(template.New("").Parse(`
<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<script>  
window.addEventListener("load", function(evt) {
    var output = document.getElementById("output");
    var input = document.getElementById("input");
    var ws;
    var print = function(message) {
        var d = document.createElement("div");
        d.textContent = message;
        output.appendChild(d);
        output.scroll(0, output.scrollHeight);
    };
    document.getElementById("open").onclick = function(evt) {
        if (ws) {
            return false;
        }
        ws = new WebSocket("{{.}}");
        ws.onopen = function(evt) {
            print("OPEN");
        }
        ws.onclose = function(evt) {
            print("CLOSE");
            ws = null;
        }
        ws.onmessage = function(evt) {
            print("RESPONSE: " + evt.data);
        }
        ws.onerror = function(evt) {
            print("ERROR: " + evt.data);
        }
        return false;
    };
    document.getElementById("send").onclick = function(evt) {
        if (!ws) {
            return false;
        }
        print("SEND: " + input.value);
        ws.send(input.value);
        return false;
    };
    document.getElementById("close").onclick = function(evt) {
        if (!ws) {
            return false;
        }
        ws.close();
        return false;
    };
});
</script>
</head>
<body>
<table>
<tr><td valign="top" width="50%">
<p>Click "Open" to create a connection to the server, 
"Send" to send a message to the server and "Close" to close the connection. 
You can change the message and send multiple times.
<p>
<form>
<button id="open">Open</button>
<button id="close">Close</button>
<p><input id="input" type="text" value="Hello world!">
<button id="send">Send</button>
</form>
</td><td valign="top" width="50%">
<div id="output" style="max-height: 70vh;overflow-y: scroll;"></div>
</td></tr></table>
</body>
</html>
`))
