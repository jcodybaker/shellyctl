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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/mongoose-os/mos/common/mgrpc/codec"
	"github.com/mongoose-os/mos/common/mgrpc/frame"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	mqtt "github.com/eclipse/paho.mqtt.golang"
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
	log         *zerolog.Logger
}

func newMQTTCodec(ctx context.Context, dst string, mqttClient mqtt.Client) (codec.Codec, error) {
	co := mqttClient.OptionsReader()
	src := fmt.Sprintf("%s/rpc-resp/%s", dst, co.ClientID())
	c := &mqttCodec{
		dst:         dst,
		closeNotify: make(chan struct{}),
		ready:       make(chan struct{}),
		rchan:       make(chan frame.Frame),
		src:         src,
		subTopic:    src,
		pubTopic:    fmt.Sprintf("%s/rpc", dst),
		isTLS:       (co.Servers()[0].Scheme == "tcps"),
		subTopics:   make(map[string]bool),
		cli:         mqttClient,
		log:         log.Ctx(ctx),
	}

	if err := c.subscribe(c.subTopic + "/rpc"); err != nil {
		return nil, fmt.Errorf("subscribing to mqtt %q: %w", c.pubTopic, err)
	}
	return c, nil
}

func newMQTTConsumer(ctx context.Context, topic string, mqttClient mqtt.Client) (codec.Codec, error) {
	co := mqttClient.OptionsReader()
	c := &mqttCodec{
		closeNotify: make(chan struct{}),
		ready:       make(chan struct{}),
		rchan:       make(chan frame.Frame),
		isTLS:       (co.Servers()[0].Scheme == "tcps"),
		subTopics:   make(map[string]bool),
		cli:         mqttClient,
		log:         log.Ctx(ctx),
	}

	if err := c.subscribe(topic); err != nil {
		return nil, fmt.Errorf("subscribing to mqtt %q: %w", c.pubTopic, err)
	}
	return c, nil
}

func (c *mqttCodec) subscribe(topic string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.subTopics[topic] {
		return nil
	}
	c.log.Debug().Str("topic", topic).Msg("subscribing to mqtt topic")
	token := c.cli.Subscribe(topic, 1 /* qos */, c.onMessage)
	token.Wait()
	if err := token.Error(); err != nil {
		return fmt.Errorf("subscribing to mqtt topic: %w", err)
	}
	c.subTopics[topic] = true
	return nil
}

func (c *mqttCodec) onMessage(cli mqtt.Client, msg mqtt.Message) {
	c.log.Debug().
		Str("topic", msg.Topic()).
		Str("payload", string(msg.Payload())).
		Msg("got message")
	f := &frame.Frame{}
	if err := json.Unmarshal(msg.Payload(), &f); err != nil {
		c.log.Err(err).
			Str("payload", string(msg.Payload())).
			Str("topic", msg.Topic()).
			Msg("invalid json payload received via mqtt")
		return
	}
	c.rchan <- *f
}

func (c *mqttCodec) Close() {
	c.closeOnce.Do(func() {
		var topics []string
		for t := range c.subTopics {
			topics = append(topics, t)
		}
		c.log.Debug().Strs("topics", topics).Msg("closing mqtt rpc channel; unsubscribing from topics")
		token := c.cli.Unsubscribe(topics...)
		token.Wait()
		if err := token.Error(); err != nil {
			c.log.Warn().Strs("topics", topics).Err(err).Msg("unsubscribing from topics")
		}
		close(c.closeNotify)
		c.log.Debug().Strs("topics", topics).Msg("unsubscribed from topics")
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
		return nil, io.EOF
	}
}

func (c *mqttCodec) Send(ctx context.Context, f *frame.Frame) error {
	f.Src = c.subTopic
	f.Dst = c.dst
	msg, err := json.Marshal(f)
	if err != nil {
		return fmt.Errorf("marshalling JSON payload for mqtt: %w", err)
	}
	topic := c.pubTopic
	if topic == "" {
		topic = fmt.Sprintf("%s/rpc", f.Dst)
	}
	c.log.Debug().
		Str("topic", topic).
		Str("payload", string(msg)).
		Msg("sending rpc via mqtt")
	token := c.cli.Publish(topic, 1 /* qos */, false /* retained */, msg)
	token.Wait()
	if err := token.Error(); err != nil {
		return fmt.Errorf("mqtt publish error: %w", err)
	}

	return nil
}

func (c *mqttCodec) SetOptions(opts *codec.Options) error {
	return errors.New("SetOptions not implemented")
}
