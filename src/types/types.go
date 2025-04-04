package types

import (
	"fmt"
	"net"
	"log"
)

type Channel struct {
	Name    string
	Members map[string]*Client
}

func (c *Channel) Send(sender *Client, message string) {
	log.Printf("Sending message to channel %s: %s", c.Name, message)
	// Send message to all members of the channel
	for _, member := range c.Members {
        if member.String() == sender.String() {
            continue
        }
		member.Send(sender.String(), fmt.Sprintf("PRIVMSG %s :%s", c.Name, message))
	}
}

// Joins a client to the channel
func (c *Channel) Join(client *Client) {
	log.Printf("%s joined channel: %s", client.String(), c)
	client.Channels[c.Name] = c
	client.Send(client.String(), fmt.Sprintf("JOIN :%s", c.Name))
    c.Send(client, fmt.Sprintf("%s has joined the channel", client.Nick))
}

// Part removes a client from the channel
func (c *Channel) Part(client *Client) {
	client.Send(client.String(), fmt.Sprintf("PART :%s", c.Name))
	delete(client.Channels, c.Name)
	c.Send(client, fmt.Sprintf("%s has left the channel", client.Nick))
}

func (c *Channel) GetClients() []string {
	clients := make([]string, 0, len(c.Members))
	for _, member := range c.Members {
		clients = append(clients, member.String())
	}
	return clients
}

// make a struct for a client
type Client struct {
	Connection     net.Conn
	Nick     string
	Username string
	Hostname string
	Realname string
	Channels map[string]*Channel
}

func (c *Client) String() string {
	return fmt.Sprintf("%s!%s@%s", c.Nick, c.Username, c.Hostname)
}

func (c *Client) Send(sender string, message string) {
	_, err := c.Connection.Write([]byte(fmt.Sprintf(":%s %s\r\n", sender, message)))
	if err != nil {
		log.Printf("Error sending message to client %s: %v", c.String(), err)
	}
}

func (c *Client) Disconnect() {
	for _, channel := range c.Channels {
		delete(channel.Members, c.Nick)
		c.Send(c.String(), fmt.Sprintf("PART %s", channel.Name))
	}
	c.Connection.Close()
	log.Printf("%s disconnected", c.String())
}