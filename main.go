package main

import (
	"bufio"
	"fmt"
	"os"
	"time"

	"github.com/garyburd/redigo/redis"
)

func exitHandle(err error, exitMsg string) {
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	} else if exitMsg != "" {
		fmt.Println(exitMsg)
		os.Exit(1)
	}
}

//Redis functions
func set(conn redis.Conn, userkey string, username string) {
	val, err := conn.Do("SET", userkey, username, "NX", "EX", 120)
	exitHandle(err, "")
	if val == nil {
		exitHandle(nil, "User already online")
	}
}

func sadd(conn redis.Conn, username string) {
	val, err := conn.Do("SADD", "users", username)
	exitHandle(err, "")
	if val == nil {
		exitHandle(nil, "User already in set")
	}
}

//listener and speaker
func listener(subChan chan string) {
	subconn, err := redis.Dial("tcp", "192.168.99.100:6379")
	exitHandle(err, "")
	defer subconn.Close()

	psc := redis.PubSubConn{Conn: subconn}
	psc.Subscribe("messages")
	for {
		switch v := psc.Receive().(type) {
		case redis.Message:
			subChan <- string(v.Data)
		case redis.Subscription:

		case error:
			return
		}
	}
}

func speaker(sayChan chan string, username string) {
	prompt := username + ">"
	bio := bufio.NewReader(os.Stdin)
	for {
		fmt.Print(prompt)
		line, _, err := bio.ReadLine()
		if err != nil {
			fmt.Println(err)
			sayChan <- "/exit"
			return
		}
		sayChan <- string(line)
	}
}

func main() {
	if len(os.Args) != 2 {
		fmt.Println("Usage: chatApp username")
		os.Exit(1)
	}
	username := os.Args[1]
	conn, err := redis.Dial("tcp", "192.168.99.100:6379")
	exitHandle(err, "")
	defer conn.Close()
	userkey := "online." + username
	set(conn, userkey, username)
	sadd(conn, username)

	tickChannel := time.NewTicker(time.Second * 60).C

	subChan := make(chan string)
	sayChan := make(chan string)
	go listener(subChan)
	go speaker(sayChan, username)
	conn.Do("PUBLISH" , "messages" , username+" has joined.")

	chatExit := false

	for !chatExit{
		select{
		case msg := <-subChan:
			fmt.Println(msg)
		case <- tickChannel:
			val, err := conn.Do("SET", userkey, username, "XX" , "EX" , "120")
			if val == nil || err != nil{
				fmt.Println("Reset failure")
				chatExit = true
			}
		case line := <-sayChan:
			switch (line){
				case "/exit" : 
					chatExit = true
				case "/who" :
					names, _ := redis.String(conn.Do("SMEMBERS" , "users"))
					for _, name := range names{
						fmt.Println(name)
					}
				default: 
					conn.Do("PUBLISH", "messages", username+ ":" +line)
			}
		default : 
			time.Sleep(100 * time.Millisecond)
		}
	}

	conn.Do("DEL", userkey)
	conn.Do("SREM", "users", username)
	conn.Do("PUBLISH", "messages", username+" has left")
}
