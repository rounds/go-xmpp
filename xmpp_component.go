// Jabber Component implementaqtion for jabber:component:accept namespace, see http://www.xmpp.org/extensions/xep-0114.html.
package xmpp

import (
	"crypto/sha1"
	"encoding/xml"
	"fmt"
	"os"
	"strings"
)

const nsComponentAccept = "jabber:component:accept"

// ComponentOptions specify additional options for component client.
type ComponentOptions struct {
	Options
}

// ComponentClient holds XMPP connection opitons for component client.
type ComponentClient struct {
	*BasicClient
}

// NewClient establishes a new component Client connection based on a set of Options.
func (o ComponentOptions) NewClient() (Client, error) {
	host := o.Host
	conn, err := connect(host, "", "")
	if err != nil {
		return nil, err
	}

	if strings.LastIndex(o.Host, ":") > 0 {
		host = host[:strings.LastIndex(o.Host, ":")]
	}

	c := &ComponentClient{
		BasicClient: &BasicClient{
			conn: conn,
			host: host,
			user: o.User,
		},
	}

	// Declare intent to be a jabber client and gather stream features.
	// XEP-0114 3. Protocol Flow.
	streamID, err := c.startStream(&o, host)
	if err != nil {
		return nil, err
	}

	// Start component handshake.
	if err := c.startHandshake(streamID, o.Password); err != nil {
		return nil, err
	}

	// Recreate the XML decoder with a default client namespace.
	if o.Debug {
		c.p = xml.NewDecoder(tee{c.conn, os.Stderr})
	} else {
		c.p = xml.NewDecoder(c.conn)
	}

	return c, nil
}

// NewComponentClient creates a new connection to a host given as "hostname" or "hostname:port"
// and initializes a component client.
func NewComponentClient(host, cname, secret string, debug bool) (Client, error) {
	opts := ComponentOptions{
		Options: Options{
			Host:     host,
			User:     cname,
			Password: secret,
			Debug:    debug,
			Session:  false,
		},
	}
	return opts.NewClient()
}

// startStream creates a new jabber:component:accept stream and returns stream ID.
func (c *ComponentClient) startStream(o *ComponentOptions, domain string) (string, error) {
	if o.Debug {
		c.p = xml.NewDecoder(tee{c.conn, os.Stderr})
	} else {
		c.p = xml.NewDecoder(c.conn)
	}

	_, err := fmt.Fprintf(c.conn, "<?xml version='1.0'?><stream:stream to='%s' xmlns='%s' xmlns:stream='%s' version='1.0'>\n",
		xmlEscape(domain), nsComponentAccept, nsStream)
	if err != nil {
		return "", err
	}

	// We expect the server to start a <stream>.
	se, err := nextStart(c.p)
	if err != nil {
		return "", err
	}
	if se.Name.Space != nsStream || se.Name.Local != "stream" {
		return "", fmt.Errorf("expected <stream> but got <%v> in %v", se.Name.Local, se.Name.Space)
	}

	// Find the stream id.
	var streamID string
	for _, attr := range se.Attr {
		if attr.Name.Local == "id" {
			streamID = attr.Value
			break
		}
	}

	return streamID, nil
}

// startHandshake starts the component handshake on the provided stream and
// checks the authentication results.
func (c *ComponentClient) startHandshake(streamID, secret string) error {
	hash := sha1.New()
	hash.Write([]byte(streamID))
	hash.Write([]byte(secret))

	// Send handshake.
	if _, err := fmt.Fprintf(c.conn, "<handshake>%x</handshake>\n", hash.Sum(nil)); err != nil {
		return err
	}

	// Read handshake reply.
	se, err := nextStart(c.p)
	if err != nil {
		return err
	}

	if se.Name.Local != "handshake" {
		if se.Name.Local == "error" {
			se, err = nextStart(c.p)
			if err != nil {
				return err
			}
			return fmt.Errorf("handshake error: %s", se.Name.Local)
		}
		return fmt.Errorf("unexpected handshake error, expected <handshake> but got <%s>", se.Name.Local)
	}

	// Skip handshake reply.
	//if err := c.p.Skip(); err != nil {
	//	return err
	//}

	return nil
}
