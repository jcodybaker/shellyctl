package discovery

import (
	"fmt"
	"net/url"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/juju/errors"
	"golang.org/x/exp/rand"
	glog "k8s.io/klog/v2"
)

func searchMQTT() {
	// 1) Subscribe to '+/status'
	// 2) Subscribe to '+/online'; remove 'off' devices when lost
	// 3) wait discovery timeout EXACTLY once
	// 3) return current known device list.
}

func MQTTClientOptsFromURL(us, clientID, user, pass string) (*mqtt.ClientOptions, string, error) {
	u, err := url.Parse(us)
	if err != nil {
		return nil, "", errors.Trace(err)
	}

	if clientID == "" {
		clientID = fmt.Sprintf("shellyctl-%d", rand.Int31())
	}

	topic := u.Path[1:]

	u.Path = ""
	if u.Scheme == "mqtts" {
		u.Scheme = "tcps"
		if u.Port() == "" {
			u.Host = fmt.Sprintf("%s:%d", u.Host, 8883)
		}
	} else {
		u.Scheme = "tcp"
		if u.Port() == "" {
			u.Host = fmt.Sprintf("%s:%d", u.Host, 1883)
		}
	}
	broker := u.String()
	glog.V(1).Infof("Connecting %s to %s", clientID, broker)

	opts := mqtt.NewClientOptions()
	opts.AddBroker(broker)
	opts.SetClientID(clientID)
	if u.User != nil {
		user = u.User.Username()
		passwd, isset := u.User.Password()
		if isset {
			pass = passwd
		}
	}
	opts.SetUsername(user)
	opts.SetPassword(pass)

	return opts, topic, nil
}
