// This file is lifted from github.com/mongoose-os/mos/common/mgrpc/codec/mqtt.go at
// commit a532b7393b24ca3aa09338352b81a195ce3cec52.
// It has been modified to:
// - Support using a single MQTT client connection for multiple devices.
// - Use zerolog
//
// Original License:
//
// Copyright (c) 2014-2019 Cesanta Software Limited
// All rights reserved
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package discovery

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"sync"

	"github.com/mongoose-os/mos/common/mgrpc/codec"
	"github.com/mongoose-os/mos/common/mgrpc/frame"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/juju/errors"
	glog "k8s.io/klog/v2"
)

type mqttCodec struct {
	src         string
	dst         string
	closeNotify chan struct{}
	ready       chan struct{}
	rchan       chan frame.Frame
	cli         mqtt.Client
	closeOnce   sync.Once
	isTLS       bool
	pubTopic    string
	subTopic    string
	subTopics   map[string]bool
	mu          sync.Mutex
}

func MQTT(dst string, tlsConfig *tls.Config, co *MQTTCodecOptions) (codec.Codec, error) {
	opts, topic, err := MQTTClientOptsFromURL(dst, co.ClientID, co.User, co.Password)
	if err != nil {
		return nil, errors.Trace(err)
	}
	if tlsConfig != nil {
		opts.SetTLSConfig(tlsConfig)
	}

	u, _ := url.Parse(dst)

	c := &mqttCodec{
		dst:         topic,
		closeNotify: make(chan struct{}),
		ready:       make(chan struct{}),
		rchan:       make(chan frame.Frame),
		src:         co.Src,
		pubTopic:    co.PubTopic,
		subTopic:    co.SubTopic,
		isTLS:       (u.Scheme == "mqtts"),
		subTopics:   make(map[string]bool),
	}
	if c.src == "" {
		c.src = opts.ClientID
	}

	opts.SetConnectionLostHandler(c.onConnectionLost)

	c.cli = mqtt.NewClient(opts)
	token := c.cli.Connect()
	token.Wait()
	if err := token.Error(); err != nil {
		return nil, errors.Annotatef(err, "MQTT connect error")
	}

	if c.subTopic != "" {
		err = c.subscribe(c.subTopic)
	}

	return c, errors.Trace(err)
}

func (c *mqttCodec) subscribe(topic string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.subTopics[topic] {
		return nil
	}
	glog.V(1).Infof("Subscribing to [%s]", topic)
	token := c.cli.Subscribe(topic, 1 /* qos */, c.onMessage)
	token.Wait()
	if err := token.Error(); err != nil {
		return errors.Annotatef(err, "MQTT subscribe error")
	}
	c.subTopics[topic] = true
	return nil
}

func (c *mqttCodec) onMessage(cli mqtt.Client, msg mqtt.Message) {
	glog.V(4).Infof("Got MQTT message, topic [%s], message [%s]", msg.Topic(), msg.Payload())
	f := &frame.Frame{}
	if err := json.Unmarshal(msg.Payload(), &f); err != nil {
		glog.Errorf("Invalid json (%s): %+v", err, msg.Payload())
		return
	}
	c.rchan <- *f
}

func (c *mqttCodec) onConnectionLost(cli mqtt.Client, err error) {
	glog.Errorf("Lost conection to MQTT broker: %s", err)
	c.Close()
}

func (c *mqttCodec) Close() {
	c.closeOnce.Do(func() {
		glog.V(1).Infof("Closing %s", c)
		close(c.closeNotify)
		c.cli.Disconnect(0)
	})
}

func (c *mqttCodec) CloseNotify() <-chan struct{} {
	return c.closeNotify
}

func (c *mqttCodec) String() string {
	return fmt.Sprintf("[mqttCodec to %s]", c.dst)
}

func (c *mqttCodec) Info() codec.ConnectionInfo {
	return codec.ConnectionInfo{
		IsConnected: c.cli.IsConnected(),
		TLS:         c.isTLS,
		RemoteAddr:  c.dst,
	}
}

func (c *mqttCodec) MaxNumFrames() int {
	return -1
}

func (c *mqttCodec) Recv(ctx context.Context) (*frame.Frame, error) {
	select {
	case f := <-c.rchan:
		return &f, nil
	case <-c.closeNotify:
		return nil, errors.Trace(io.EOF)
	}
}

func (c *mqttCodec) Send(ctx context.Context, f *frame.Frame) error {
	if f.Dst == "" {
		f.Dst = c.dst
	}
	if c.subTopic == "" {
		f.Src = fmt.Sprintf("%s/rpc-resp/%s", f.Dst, c.src)
		if err := c.subscribe(fmt.Sprintf("%s/rpc", f.Src)); err != nil {
			return errors.Trace(err)
		}
	} else {
		f.Src = c.src
	}
	msg, err := json.Marshal(f)
	if err != nil {
		return errors.Trace(err)
	}
	topic := c.pubTopic
	if topic == "" {
		topic = fmt.Sprintf("%s/rpc", f.Dst)
	}
	glog.V(4).Infof("Sending [%s] to [%s]", msg, topic)
	token := c.cli.Publish(topic, 1 /* qos */, false /* retained */, msg)
	token.Wait()
	if err := token.Error(); err != nil {
		return errors.Annotatef(err, "MQTT publish error")
	}
	return nil
}

func (c *mqttCodec) SetOptions(opts *codec.Options) error {
	return errors.NotImplementedf("SetOptions")
}
