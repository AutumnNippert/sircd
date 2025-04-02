package main

import (
	"fmt"
	"net"
	"strings"
	"time"
	"log"
	"os"
	"io"
)

var (
	Version = "0.1"
	port    = "6667"
	host    = "0.0.0.0"
	logFile *os.File

	clientMap = make(map[string]*Client)
	connMap   = make(map[net.Conn]*Client)
	// channels map
	channels = make(map[string]*Channel)
)

type Channel struct {
	name    string
	members map[string]*Client
}

// make a struct for a client
type Client struct {
	conn     net.Conn
	nick     string
	username string
	hostname string
	realname string
	channels map[string]*Channel
}

// make a str function for Client
func (c *Client) String() string {
	return fmt.Sprintf("%s!%s@%s", c.nick, c.username, c.hostname)
}

func init_logging(){
	// Open log file
	logFile, err := os.OpenFile("irc_server.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		fmt.Println("Error opening log file:", err)
		return
	}

	multiWriter := io.MultiWriter(os.Stdout, logFile)

	// Set output to log file
	log.SetOutput(multiWriter)

	// Set log flags
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	log.Println("Logging started")
}

func listen() {
	listener, err := net.Listen("tcp", host+":"+port)
	if err != nil {
		log.Fatal(err)
	}
	defer listener.Close()

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Println(err)
			continue
		}
		go handleConnection(conn)
	}
}

func getDataElseDie(conn net.Conn) ([]byte, error) {
	data := make([]byte, 1024)
	n, err := conn.Read(data)
	if err != nil {
		log.Println("Error reading data:", err)
		return nil, err
	}
	if n == 0 {
		log.Println("Connection closed by client")
		return nil, nil
	}
	return data[:n], nil

}

func handleConnection(connection net.Conn) {
	// Set a read timeout
	connection.SetReadDeadline(time.Now().Add(5 * time.Minute))

	// Read the incoming data
	buffer := make([]byte, 1024)
	
	err := handleNewUser(connection)
	if err != nil {
		log.Println("Error handling new user:", err)
		connection.Close()
		return
	}

	// messaging loop
	for {
		n, err := connection.Read(buffer)
		if err != nil {
			log.Println(err)
			break
		}

		data := strings.TrimSpace(string(buffer[:n]))
		if data == "" {
			continue
		}
		
		// Parse the command
		client := connMap[connection]
		if client == nil {
			log.Println("Client not found")
			break
		}
		log.Printf("Received data from %s: %s", client.String(), data)
		parseCommand(data, client)
	}
}

func handleNewUser(connection net.Conn) error {
	nick_data, err := getDataElseDie(connection)
	if err != nil {
		log.Println("Error getting NICK data:", err)
		connection.Close()
		return err
	}
	log.Println("NICK data:", string(nick_data))
	nick := strings.TrimSpace(string(nick_data))
	if strings.HasPrefix(nick, "NICK") {
		nick = strings.TrimSpace(strings.Split(nick, " ")[1])
	} else {
		log.Println("Invalid NICK command")
		return err
	}

	// if NICk already exists, send error message
	if _, exists := clientMap[nick]; exists {
		log.Println("Nick already in use:", nick)
		connection.Write([]byte("ERROR :Nickname already in use\r\n"))
		connection.Close()
		return fmt.Errorf("nickname already in use")
	}

	user_data, err := getDataElseDie(connection)
	if err != nil {
		log.Println("Error getting USER data:", err)
		return err
	}

	log.Println("USER data:", string(user_data))
	tokens := strings.Split(string(user_data), " ")

	var userName string
	var realName string
	if tokens[0] != "USER" {
		log.Println("Invalid USER command")
		return err
	}

	if len(tokens) >= 5 {
		userName = strings.TrimSpace(tokens[1])
		realName = strings.TrimSpace(strings.Join(tokens[4:], " "))
	} else if len(tokens) >= 4 {
		userName = strings.TrimSpace(tokens[1])
		realName = strings.TrimSpace(tokens[3])

	} else {
		log.Println("Invalid USER command")
		return err
	}

	// Create a new client
	client := &Client{
		conn:     connection,
		nick:     nick,
		username: userName,
		hostname: connection.RemoteAddr().String(),
		realname: realName,
		channels: make(map[string]*Channel),
	}

	log.Printf("%s connected", client.String())

	// send back a 001 message
	connection.Write([]byte(fmt.Sprintf(":%s 001 %s :Welcome to the IRC server\r\n", host, client.nick)))
	log.Printf("Sent: :%s 001 %s :Welcome to the IRC server", host, client.nick)

	// Add the client to the map
	clientMap[client.nick] = client
	connMap[connection] = client

	return nil

}

func createChannel(name string) *Channel {
	log.Printf("Creating channel: %s", name)
	// Create a new channel
	channel := &Channel{
		name:    name,
		members: make(map[string]*Client),
	}
	channels[name] = channel
	return channel
}

func addUserToChannel(channel *Channel, client *Client) {
	log.Printf("%s joined channel: %s", client.String(), channel)
	channels[channel.name].members[client.nick] = client
	client.conn.Write([]byte(fmt.Sprintf(":%s JOIN %s\r\n", client.String(), channel.name)))
	client.channels[channel.name] = channel
	log.Printf("%s joined channel %s", client.String(), channel.name)

	// Send a message to all members of the channel
	message := fmt.Sprintf("%s has joined the channel", client.nick)
	sendMessageToChannel(channel, message)
}

func disconnectUser(client *Client) {
	// disconnects the user from all of their channels
	for _, channel := range client.channels {
		delete(channel.members, client.nick)
		client.conn.Write([]byte(fmt.Sprintf(":%s PART %s\r\n", client.String(), channel.name)))
	}
	delete(clientMap, client.nick)
	delete(connMap, client.conn)
	client.conn.Close()
	log.Printf("%s disconnected", client.String())
}

func sendMessageToChannel(channel *Channel, message string) {
	log.Printf("Sending message to channel %s: %s", channel.name, message)
	// Send message to all members of the channel
	for _, member := range channel.members {
		member.conn.Write([]byte(fmt.Sprintf(":%s PRIVMSG %s :%s\r\n", member.String(), channel.name, message)))
	}
}

func parseCommand(str string, client *Client) {
	// valid commands:
	/*
	PRIVMSG <target> :<message>
	PING <server>
	PONG <server>
	JOIN <channel>
	PART <channel>
	QUIT :<message>
	*/

	tokens := strings.Split(str, " ")
	if len(tokens) == 0 {
		return
	}

	switch tokens[0] {
	case "EOF":
		log.Println("EOF received, closing connection")
		// for each channel they are in, tell them they have left
		for _, channel := range channels {
			if _, exists := channel.members[client.nick]; exists {
				delete(channel.members, client.nick)
				client.conn.Write([]byte(fmt.Sprintf(":%s PART %s\r\n", client.String(), channel.name)))
			}
		}
		client.conn.Close()
		delete(clientMap, client.nick)
		delete(connMap, client.conn)
		log.Printf("%s disconnected", client.String())
	case "PRIVMSG":
		// check if the target is a channel or a user
		target := tokens[1]
		message := strings.Join(tokens[2:], " ")
		// get rid of leading :
		if strings.HasPrefix(message, ":") {
			message = strings.TrimPrefix(message, ":")
		}
		if strings.HasPrefix(target, "#") {
			// send message to channel
			channel, exists := channels[target]
			if !exists {
				log.Println("Channel not found:", target)
				client.conn.Write([]byte(fmt.Sprintf("ERROR :Channel %s not found\r\n", target)))
				return
			}
			sendMessageToChannel(channel, message)
		} else {
			// send message to user
			client, exists := clientMap[target]
			if exists {
				client.conn.Write([]byte(fmt.Sprintf("PRIVMSG %s :%s\r\n", client.nick, message)))
			} else {
				log.Println("User not found:", target)
			}
		}
	case "PING":
		// respond to ping
		response := strings.Replace(str, "PING", "PONG", 1)
		for _, client := range clientMap {
			client.conn.Write([]byte(response))
		}
	case "PONG":
		// respond to pong
		for _, client := range clientMap {
			client.conn.Write([]byte(str))
		}
	case "JOIN":
		// join a channel
		channel := tokens[1]
		if _, exists := channels[channel]; !exists {
			channels[channel] = createChannel(channel)
		}
		channelObject := channels[channel]
		addUserToChannel(channelObject, client)
		client.conn.Write([]byte(fmt.Sprintf("JOIN %s\r\n", channel)))
	case "PART":
		// part a channel
		channel := tokens[1]
		if _, exists := channels[channel]; exists {
			delete(channels[channel].members, client.nick)
			client.conn.Write([]byte(fmt.Sprintf("PART %s\r\n", channel)))
			if len(channels[channel].members) == 0 {
				delete(channels, channel)
			}
			// remove the channel from the client's list of channels
			delete(client.channels, channel)
			// send message to all members of the channel
			message := fmt.Sprintf("%s has left the channel", client.nick)
			sendMessageToChannel(channels[channel], message)
		} else {
			client.conn.Write([]byte(fmt.Sprintf("ERROR :You are not in channel %s\r\n", channel)))
		}
	case "QUIT":
		// quit the server
		message := strings.Join(tokens[1:], " ")
		client.conn.Write([]byte(fmt.Sprintf("QUIT :%s\r\n", message)))
		client.conn.Close()
		delete(clientMap, client.nick)
		delete(connMap, client.conn)
		log.Printf("%s disconnected", client.String())
	default:
		log.Println("Unknown command:", tokens[0])
		client.conn.Write([]byte(fmt.Sprintf("ERROR :Unknown command %s\r\n", tokens[0])))
	}
}

func main() {
	fmt.Println("Starting IRC server on", host+":"+port)
	fmt.Println("Version:", Version)

	// Initialize logging
	init_logging()

	// Start listening for connections
	listen()
}