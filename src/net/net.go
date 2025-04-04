package net

import (
	"log"
	"net"
	"strings"
	"time"
	"fmt"
	"io"

	. "sircd/src/types"
	. "sircd/src/util"
)

var (
	clientMap = make(map[string]*Client)
	connMap   = make(map[net.Conn]*Client)
	// channels map
	channels = make(map[string]*Channel)
)

func Listen(host string, port string) {
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
		// check if the connection is still open
		if connection == nil {
			log.Println("Connection is nil, closing")
			break
		}

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
	buffer := make([]byte, 1024)
	// Read the incoming data
	connection.SetReadDeadline(time.Now().Add(5 * time.Minute))

	n, err := connection.Read(buffer)
	if n == 0 {
		log.Println("Connection closed by client")
		connection.Close()
		return nil
	}
	if err != nil {
		log.Println("Error getting NICK data:", err)
		connection.Close()
		return err
	}
	log.Println("NICK data:", string(buffer))

    // check if CAP LS 302 or any other version is sent, this guages the client version of irc
    if strings.HasPrefix(string(buffer), "CAP") {
        // check if the client is requesting a Version
        if strings.Contains(string(buffer), "LS") {
        }
        n, err = connection.Read(buffer)
		if n == 0 {
			log.Println("Connection closed by client")
			connection.Close()
			return nil
		}
        if err != nil {
            log.Println("Error getting NICK data:", err)
            connection.Close()
            return err
        }
        log.Println("NICK data:", string(buffer))
    }

	nick := strings.TrimSpace(string(buffer))
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

	n, err = connection.Read(buffer)
	if n == 0 {
		log.Println("Connection closed by client")
		connection.Close()
		return nil
	}
	if err != nil {
		// check if err is EOF
		if err == io.EOF {
			log.Println("Connection closed by client")
			// check if client is in the map
			client := connMap[connection]
			if client != nil {
				disconnectUser(client)
			}
			return nil
		}

		log.Println("Error getting USER data:", err)
		return err
	}

	log.Println("USER data:", string(buffer))
	tokens := strings.Split(string(buffer), " ")

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
		Connection:     connection,
		Nick:     nick,
		Username: userName,
		Hostname: connection.RemoteAddr().String(),
		Realname: realName,
		Channels: make(map[string]*Channel),
	}

	log.Printf("%s connected", client.String())

    // Send the welcome message
    sendInitMessages(client)

	// Add the client to the map
	clientMap[client.Nick] = client
	connMap[connection] = client

	return nil

}

func PRIVMSG(client *Client, tokens []string) {
	target := tokens[1]
	message := strings.Join(tokens[2:], " ")

	// CASE 1: target is a channel
	if strings.HasPrefix(target, "#") {
		channel, exists := client.Channels[target]
		if !exists {
			log.Println("Channel not found:", target)
			// client.conn.Write([]byte(fmt.Sprintf("ERROR :Channel %s not found", target)))
			client.Send(HOST, fmt.Sprintf("ERROR :Channel %s not found", target))
			return
		}
		channel.Send(client, message)
	} else { // CASE 2: target is a user
		client, exists := clientMap[target]
		if exists {
			client.Send(client.String(), fmt.Sprintf("PRIVMSG %s :%s", message))
		} else {
			log.Println("User not found:", target)
		}
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
		client.Disconnect()
		delete(clientMap, client.Nick)
		delete(connMap, client.Connection)
		log.Printf("%s disconnected", client.String())
	case "PRIVMSG":
		// send a message to a user or channel
		PRIVMSG(client, tokens)
	case "PING":
		client.Send(HOST, fmt.Sprintf("PONG %s", tokens[1]))
	case "JOIN":
		// join a channel
		channel := tokens[1]
		if _, exists := channels[channel]; !exists {
			channels[channel] = createChannel(channel)
		}
		channelObject := channels[channel]
		if _, exists := channelObject.Members[client.Nick]; !exists {
			channels[channel].Members[client.Nick] = client
			channelObject.Join(client)
		} else {
			client.Send(HOST, fmt.Sprintf("ERROR :You are already in channel %s", channel))
		}
	case "PART":
		channel := tokens[1]
		if channelObject, exists := channels[channel]; exists {
			channelObject.Part(client)
			delete(channelObject.Members, client.Nick)
			if len(channelObject.Members) == 0 {
				delete(channels, channel)
			}
		} else {
			client.Send(HOST, fmt.Sprintf("ERROR :You are not in channel %s", channel))
		}
	case "QUIT":
		// quit the server
		client.Disconnect()
		delete(clientMap, client.Nick)
		delete(connMap, client.Connection)
		log.Printf("%s disconnected", client.String())
	default:
		log.Println("Unknown command:", tokens[0])
		client.Send(HOST, fmt.Sprintf("ERROR :Unknown command %s", tokens[0]))
	}
}

func sendInitMessages(client *Client) {
    client.Send(HOST, fmt.Sprintf("001 %s :Welcome to the IRC server", client.Nick))
    client.Send(HOST, fmt.Sprintf("002 %s :Your host is %s, running version %s", client.Nick, HOST, VERSION))
    client.Send(HOST, fmt.Sprintf("003 %s :This server was created %s", client.Nick, STARTUPTIME))
    client.Send(HOST, fmt.Sprintf("251 %s :There are 0 users and 0 invisible on 1 servers", client.Nick))
    client.Send(HOST, fmt.Sprintf("253 %s 1 :unknown connections", client.Nick))
    client.Send(HOST, fmt.Sprintf("254 %s 0 :channels formed", client.Nick))
    client.Send(HOST, fmt.Sprintf("255 %s :I have 0 clients and 0 servers", client.Nick))
    client.Send(HOST, fmt.Sprintf("265 %s :Current local users: 0  Max: 1", client.Nick))
    client.Send(HOST, fmt.Sprintf("266 %s :Current global users: 0  Max: 1", client.Nick))
    client.Send(HOST, fmt.Sprintf("375 %s :%s message of the day", client.Nick, HOST))
    client.Send(HOST, fmt.Sprintf("372 %s : You know the drill", client.Nick))
    client.Send(HOST, fmt.Sprintf("372 %s : ", client.Nick))
    client.Send(HOST, fmt.Sprintf("376 %s :End of message of the day.", client.Nick))
}
 
func createChannel(name string) *Channel {
	log.Printf("Creating channel: %s", name)
	// Create a new channel
	channel := &Channel{
		Name:    name,
		Members: make(map[string]*Client),
	}
	channels[name] = channel
	return channel
}

func disconnectUser(client *Client) {
	delete(clientMap, client.Nick)
	delete(connMap, client.Connection)
	client.Disconnect()
}