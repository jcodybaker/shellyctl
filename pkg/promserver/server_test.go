package promserver

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jcodybaker/shellyctl/pkg/discovery"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"
)

func TestCollect(t *testing.T) {
	tcs := []struct {
		name     string
		d1Status string
		d1Config string
		expect   string
	}{
		{
			name: "simple switch (Plug-US)",
			d1Status: `{
				  "ble": {},
				  "cloud": {
					"connected": true
				  },
				  "mqtt": {
					"connected": false
				  },
				  "switch:0": {
					"id": 0,
					"source": "HTTP_in",
					"output": false,
					"apower": 0,
					"voltage": 121.9,
					"current": 0,
					"aenergy": {
					  "total": 24.262,
					  "by_minute": [
						0,
						0,
						0
					  ],
					  "minute_ts": 1704575558
					},
					"temperature": {
					  "tC": 48.3,
					  "tF": 118.9
					}
				  },
				  "sys": {
					"mac": "$MAC_DEVICE_1",
					"restart_required": false,
					"time": "16:12",
					"unixtime": 1704575559,
					"uptime": 861845,
					"ram_size": 246496,
					"ram_free": 144336,
					"fs_size": 458752,
					"fs_free": 151552,
					"cfg_rev": 13,
					"kvs_rev": 0,
					"schedule_rev": 0,
					"webhook_rev": 0,
					"available_updates": {},
					"reset_reason": 3
				  },
				  "wifi": {
					"sta_ip": "192.168.1.10",
					"status": "got ip",
					"ssid": "INTERNET",
					"rssi": -52
				  },
				  "ws": {
					"connected": false
				  }
				}`,
			d1Config: `
				{
				  "ble": {
					"enable": false,
					"rpc": {
					  "enable": true
					},
					"observer": {
					  "enable": false
					}
				  },
				  "cloud": {
					"enable": true,
					"server": "shelly-78-eu.shelly.cloud:6022/jrpc"
				  },
				  "mqtt": {
					"enable": false,
					"server": null,
					"client_id": "shellyplugus-$MAC_DEVICE_1",
					"user": null,
					"ssl_ca": null,
					"topic_prefix": "shellyplugus-$MAC_DEVICE_1",
					"rpc_ntf": true,
					"status_ntf": false,
					"use_client_cert": false,
					"enable_rpc": true,
					"enable_control": true
				  },
				  "switch:0": {
					"id": 0,
					"name": null,
					"initial_state": "off",
					"auto_on": false,
					"auto_on_delay": 60,
					"auto_off": false,
					"auto_off_delay": 60,
					"power_limit": 4480,
					"voltage_limit": 280,
					"autorecover_voltage_errors": false,
					"current_limit": 16
				  },
				  "sys": {
					"device": {
					  "name": null,
					  "mac": "$MAC_DEVICE_1",
					  "fw_id": "20231219-133953/1.1.0-g34b5d4f",
					  "discoverable": true,
					  "eco_mode": false
					},
					"location": {
					  "tz": "America/Detroit",
					  "lat": 44.75999,
					  "lon": -85.61584
					},
					"debug": {
					  "level": 2,
					  "file_level": null,
					  "mqtt": {
						"enable": false
					  },
					  "websocket": {
						"enable": false
					  },
					  "udp": {
						"addr": null
					  }
					},
					"ui_data": {},
					"rpc_udp": {
					  "dst_addr": null,
					  "listen_port": null
					},
					"sntp": {
					  "server": "time.google.com"
					},
					"cfg_rev": 13
				  },
				  "wifi": {
					"ap": {
					  "ssid": "ShellyPlugUS-$MAC_DEVICE_1",
					  "is_open": true,
					  "enable": false,
					  "range_extender": {
						"enable": false
					  }
					},
					"sta": {
					  "ssid": "INTERNET",
					  "is_open": false,
					  "enable": true,
					  "ipv4mode": "dhcp",
					  "ip": null,
					  "netmask": null,
					  "gw": null,
					  "nameserver": null
					},
					"sta1": {
					  "ssid": null,
					  "is_open": true,
					  "enable": false,
					  "ipv4mode": "dhcp",
					  "ip": null,
					  "netmask": null,
					  "gw": null,
					  "nameserver": null
					},
					"roam": {
					  "rssi_thr": -80,
					  "interval": 60
					}
				  },
				  "ws": {
					"enable": false,
					"server": null,
					"ssl_ca": "ca.pem"
				  }
				}
			  `,
			expect: `
# HELP shelly_status_component_error 1 if the error condition ("error" label) is active; 0 or omitted if the error has cleared.
# TYPE shelly_status_component_error gauge
shelly_status_component_error{component="switch",component_name="switch:0",device_name="000000000001",error="overpower",id="0",instance="$INSTANCE_DEVICE_1",mac="000000000001"} 0
shelly_status_component_error{component="switch",component_name="switch:0",device_name="000000000001",error="overtemp",id="0",instance="$INSTANCE_DEVICE_1",mac="000000000001"} 0
shelly_status_component_error{component="switch",component_name="switch:0",device_name="000000000001",error="overvoltage",id="0",instance="$INSTANCE_DEVICE_1",mac="000000000001"} 0
shelly_status_component_error{component="switch",component_name="switch:0",device_name="000000000001",error="undervoltage",id="0",instance="$INSTANCE_DEVICE_1",mac="000000000001"} 0
# HELP shelly_status_current_amperes Last measured current in amperes.
# TYPE shelly_status_current_amperes gauge
shelly_status_current_amperes{component="switch",component_name="switch:0",device_name="000000000001",id="0",instance="$INSTANCE_DEVICE_1",mac="000000000001"} 0
# HELP shelly_status_instantaneous_active_power_watts Last measured instantaneous active power (in Watts) delivered to the attached load (shown if applicable)
# TYPE shelly_status_instantaneous_active_power_watts gauge
shelly_status_instantaneous_active_power_watts{component="switch",component_name="switch:0",device_name="000000000001",id="0",instance="$INSTANCE_DEVICE_1",mac="000000000001"} 0
# HELP shelly_status_switch_output_on 1 if the switch output is on; 0 if it is off.
# TYPE shelly_status_switch_output_on gauge
shelly_status_switch_output_on{component_name="switch:0",device_name="000000000001",id="0",instance="$INSTANCE_DEVICE_1",mac="000000000001"} 0
# HELP shelly_status_temperature_celsius Temperature in degrees celsius.
# TYPE shelly_status_temperature_celsius gauge
shelly_status_temperature_celsius{component="switch",component_name="switch:0",device_name="000000000001",id="0",instance="$INSTANCE_DEVICE_1",mac="000000000001"} 48.3
# HELP shelly_status_temperature_fahrenheit Temperature in degrees farenheit.
# TYPE shelly_status_temperature_fahrenheit gauge
shelly_status_temperature_fahrenheit{component="switch",component_name="switch:0",device_name="000000000001",id="0",instance="$INSTANCE_DEVICE_1",mac="000000000001"} 118.9
# HELP shelly_status_total_energy_watt_hours Total energy consumed in Watt-hours.
# TYPE shelly_status_total_energy_watt_hours counter
shelly_status_total_energy_watt_hours{component="switch",component_name="switch:0",device_name="000000000001",id="0",instance="$INSTANCE_DEVICE_1",mac="000000000001"} 24.262
# HELP shelly_status_voltage Last measured voltage.
# TYPE shelly_status_voltage gauge
shelly_status_voltage{component="switch",component_name="switch:0",device_name="000000000001",id="0",instance="$INSTANCE_DEVICE_1",mac="000000000001"} 121.9
`,
		},
		{
			name: "Shelly Pro PM4",
			d1Status: `
			{
			"ble": {},
			"cloud": {
				"connected": true
			},
			"eth": {
				"ip": null
			},
			"input:0": {
				"id": 0,
				"state": false
			},
			"input:1": {
				"id": 1,
				"state": false
			},
			"input:2": {
				"id": 2,
				"state": false
			},
			"input:3": {
				"id": 3,
				"state": false
			},
			"mqtt": {
				"connected": false
			},
			"switch:0": {
				"id": 0,
				"source": "timer",
				"output": false,
				"apower": 0,
				"voltage": 119.8,
				"freq": 60,
				"current": 0,
				"pf": 0,
				"aenergy": {
				"total": 1759.789,
				"by_minute": [
					0,
					0,
					0
				],
				"minute_ts": 1704579286
				},
				"ret_aenergy": {
				"total": 0,
				"by_minute": [
					0,
					0,
					0
				],
				"minute_ts": 1704579286
				},
				"temperature": {
				"tC": 44.4,
				"tF": 111.9
				}
			},
			"switch:1": {
				"id": 1,
				"source": "HTTP_in",
				"output": true,
				"apower": 84.4,
				"voltage": 119.8,
				"freq": 60,
				"current": 1.135,
				"pf": 0.62,
				"aenergy": {
				"total": 121397.383,
				"by_minute": [
					1155.867,
					1475.602,
					1475.344
				],
				"minute_ts": 1704579286
				},
				"ret_aenergy": {
				"total": 0,
				"by_minute": [
					0,
					0,
					0
				],
				"minute_ts": 1704579286
				},
				"temperature": {
				"tC": 44.4,
				"tF": 111.9
				}
			},
			"switch:2": {
				"id": 2,
				"source": "HTTP_in",
				"output": true,
				"apower": 206.9,
				"voltage": 119.8,
				"freq": 60,
				"current": 1.728,
				"pf": 1,
				"aenergy": {
				"total": 97495.339,
				"by_minute": [
					2775.534,
					3540.515,
					3539.221
				],
				"minute_ts": 1704579286
				},
				"ret_aenergy": {
				"total": 0,
				"by_minute": [
					0,
					0,
					0
				],
				"minute_ts": 1704579286
				},
				"temperature": {
				"tC": 44.4,
				"tF": 111.9
				}
			},
			"switch:3": {
				"id": 3,
				"source": "HTTP_in",
				"output": false,
				"apower": 0,
				"voltage": 119.8,
				"freq": 60,
				"current": 0,
				"pf": 0,
				"aenergy": {
				"total": 16.973,
				"by_minute": [
					0,
					0,
					0
				],
				"minute_ts": 1704579286
				},
				"ret_aenergy": {
				"total": 0,
				"by_minute": [
					0,
					0,
					0
				],
				"minute_ts": 1704579286
				},
				"temperature": {
				"tC": 44.4,
				"tF": 111.9
				}
			},
			"sys": {
				"mac": "$MAC_DEVICE_1",
				"restart_required": false,
				"time": "17:14",
				"unixtime": 1704579287,
				"uptime": 865539,
				"ram_size": 241432,
				"ram_free": 104804,
				"fs_size": 524288,
				"fs_free": 196608,
				"cfg_rev": 26,
				"kvs_rev": 1,
				"schedule_rev": 0,
				"webhook_rev": 0,
				"available_updates": {},
				"reset_reason": 3
			},
			"ui": {},
			"wifi": {
				"sta_ip": "192.168.1.24",
				"status": "got ip",
				"ssid": "INTERNET",
				"rssi": -31
			},
			"ws": {
				"connected": false
			}
			}
			`,
			d1Config: `
			{
				"ble": {
				"enable": false,
				"rpc": {
					"enable": true
				},
				"observer": {
					"enable": false
				}
				},
				"cloud": {
				"enable": true,
				"server": "shelly-78-eu.shelly.cloud:6022/jrpc"
				},
				"eth": {
				"enable": true,
				"ipv4mode": "dhcp",
				"ip": null,
				"netmask": null,
				"gw": null,
				"nameserver": null
				},
				"input:0": {
				"id": 0,
				"name": null,
				"type": "switch",
				"enable": true,
				"invert": false
				},
				"input:1": {
				"id": 1,
				"name": null,
				"type": "switch",
				"enable": true,
				"invert": false
				},
				"input:2": {
				"id": 2,
				"name": null,
				"type": "switch",
				"enable": true,
				"invert": false
				},
				"input:3": {
				"id": 3,
				"name": null,
				"type": "switch",
				"enable": true,
				"invert": false
				},
				"mqtt": {
				"enable": false,
				"server": null,
				"client_id": "shellypro4pm-$MAC_DEVICE_1",
				"user": null,
				"ssl_ca": null,
				"topic_prefix": "shellypro4pm-$MAC_DEVICE_1",
				"rpc_ntf": true,
				"status_ntf": false,
				"use_client_cert": false,
				"enable_rpc": true,
				"enable_control": true
				},
				"switch:0": {
				"id": 0,
				"name": "Change Pump",
				"in_mode": "follow",
				"initial_state": "match_input",
				"auto_on": false,
				"auto_on_delay": 60,
				"auto_off": true,
				"auto_off_delay": 5400,
				"power_limit": 4480,
				"voltage_limit": 280,
				"undervoltage_limit": 0,
				"autorecover_voltage_errors": false,
				"current_limit": 16
				},
				"switch:1": {
				"id": 1,
				"name": "Lift Pump",
				"in_mode": "follow",
				"initial_state": "on",
				"auto_on": false,
				"auto_on_delay": 60,
				"auto_off": false,
				"auto_off_delay": 450,
				"power_limit": 4480,
				"voltage_limit": 280,
				"undervoltage_limit": 0,
				"autorecover_voltage_errors": false,
				"current_limit": 16
				},
				"switch:2": {
				"id": 2,
				"name": "Heater",
				"in_mode": "follow",
				"initial_state": "match_input",
				"auto_on": false,
				"auto_on_delay": 60,
				"auto_off": false,
				"auto_off_delay": 60,
				"power_limit": 4480,
				"voltage_limit": 280,
				"undervoltage_limit": 0,
				"autorecover_voltage_errors": false,
				"current_limit": 16
				},
				"switch:3": {
				"id": 3,
				"name": "Bubbles Evac",
				"in_mode": "follow",
				"initial_state": "match_input",
				"auto_on": false,
				"auto_on_delay": 60,
				"auto_off": false,
				"auto_off_delay": 60,
				"power_limit": 4480,
				"voltage_limit": 280,
				"undervoltage_limit": 0,
				"autorecover_voltage_errors": false,
				"current_limit": 16
				},
				"sys": {
				"device": {
					"name": null,
					"mac": "$MAC_DEVICE_1",
					"fw_id": "20231219-133936/1.1.0-g34b5d4f",
					"discoverable": true,
					"eco_mode": false
				},
				"location": {
					"tz": "America/Detroit",
					"lat": 44.75999,
					"lon": -85.61584
				},
				"debug": {
					"level": 2,
					"file_level": null,
					"mqtt": {
					"enable": false
					},
					"websocket": {
					"enable": false
					},
					"udp": {
					"addr": null
					}
				},
				"ui_data": {
					"consumption_types": [
					"",
					"general",
					"",
					""
					]
				},
				"rpc_udp": {
					"dst_addr": null,
					"listen_port": null
				},
				"sntp": {
					"server": "time.google.com"
				},
				"cfg_rev": 26
				},
				"ui": {
				"idle_brightness": 30
				},
				"wifi": {
				"ap": {
					"ssid": "ShellyPro4PM-$MAC_DEVICE_1",
					"is_open": true,
					"enable": false,
					"range_extender": {
					"enable": false
					}
				},
				"sta": {
					"ssid": "INTERNET",
					"is_open": false,
					"enable": true,
					"ipv4mode": "dhcp",
					"ip": null,
					"netmask": null,
					"gw": null,
					"nameserver": null
				},
				"sta1": {
					"ssid": null,
					"is_open": true,
					"enable": false,
					"ipv4mode": "dhcp",
					"ip": null,
					"netmask": null,
					"gw": null,
					"nameserver": null
				},
				"roam": {
					"rssi_thr": -80,
					"interval": 60
				}
				},
				"ws": {
				"enable": false,
				"server": null,
				"ssl_ca": "ca.pem"
				}
			}
			`,
			expect: `# HELP shelly_status_component_error 1 if the error condition ("error" label) is active; 0 or omitted if the error has cleared.
# TYPE shelly_status_component_error gauge
shelly_status_component_error{component="input",component_name="input:0",device_name="$MAC_DEVICE_1",error="out_of_range",id="0",instance="$INSTANCE_DEVICE_1",mac="$MAC_DEVICE_1"} 0
shelly_status_component_error{component="input",component_name="input:0",device_name="$MAC_DEVICE_1",error="read",id="0",instance="$INSTANCE_DEVICE_1",mac="$MAC_DEVICE_1"} 0
shelly_status_component_error{component="input",component_name="input:1",device_name="$MAC_DEVICE_1",error="out_of_range",id="1",instance="$INSTANCE_DEVICE_1",mac="$MAC_DEVICE_1"} 0
shelly_status_component_error{component="input",component_name="input:1",device_name="$MAC_DEVICE_1",error="read",id="1",instance="$INSTANCE_DEVICE_1",mac="$MAC_DEVICE_1"} 0
shelly_status_component_error{component="input",component_name="input:2",device_name="$MAC_DEVICE_1",error="out_of_range",id="2",instance="$INSTANCE_DEVICE_1",mac="$MAC_DEVICE_1"} 0
shelly_status_component_error{component="input",component_name="input:2",device_name="$MAC_DEVICE_1",error="read",id="2",instance="$INSTANCE_DEVICE_1",mac="$MAC_DEVICE_1"} 0
shelly_status_component_error{component="input",component_name="input:3",device_name="$MAC_DEVICE_1",error="out_of_range",id="3",instance="$INSTANCE_DEVICE_1",mac="$MAC_DEVICE_1"} 0
shelly_status_component_error{component="input",component_name="input:3",device_name="$MAC_DEVICE_1",error="read",id="3",instance="$INSTANCE_DEVICE_1",mac="$MAC_DEVICE_1"} 0
shelly_status_component_error{component="switch",component_name="Bubbles Evac",device_name="$MAC_DEVICE_1",error="overpower",id="3",instance="$INSTANCE_DEVICE_1",mac="$MAC_DEVICE_1"} 0
shelly_status_component_error{component="switch",component_name="Bubbles Evac",device_name="$MAC_DEVICE_1",error="overtemp",id="3",instance="$INSTANCE_DEVICE_1",mac="$MAC_DEVICE_1"} 0
shelly_status_component_error{component="switch",component_name="Bubbles Evac",device_name="$MAC_DEVICE_1",error="overvoltage",id="3",instance="$INSTANCE_DEVICE_1",mac="$MAC_DEVICE_1"} 0
shelly_status_component_error{component="switch",component_name="Bubbles Evac",device_name="$MAC_DEVICE_1",error="undervoltage",id="3",instance="$INSTANCE_DEVICE_1",mac="$MAC_DEVICE_1"} 0
shelly_status_component_error{component="switch",component_name="Change Pump",device_name="$MAC_DEVICE_1",error="overpower",id="0",instance="$INSTANCE_DEVICE_1",mac="$MAC_DEVICE_1"} 0
shelly_status_component_error{component="switch",component_name="Change Pump",device_name="$MAC_DEVICE_1",error="overtemp",id="0",instance="$INSTANCE_DEVICE_1",mac="$MAC_DEVICE_1"} 0
shelly_status_component_error{component="switch",component_name="Change Pump",device_name="$MAC_DEVICE_1",error="overvoltage",id="0",instance="$INSTANCE_DEVICE_1",mac="$MAC_DEVICE_1"} 0
shelly_status_component_error{component="switch",component_name="Change Pump",device_name="$MAC_DEVICE_1",error="undervoltage",id="0",instance="$INSTANCE_DEVICE_1",mac="$MAC_DEVICE_1"} 0
shelly_status_component_error{component="switch",component_name="Heater",device_name="$MAC_DEVICE_1",error="overpower",id="2",instance="$INSTANCE_DEVICE_1",mac="$MAC_DEVICE_1"} 0
shelly_status_component_error{component="switch",component_name="Heater",device_name="$MAC_DEVICE_1",error="overtemp",id="2",instance="$INSTANCE_DEVICE_1",mac="$MAC_DEVICE_1"} 0
shelly_status_component_error{component="switch",component_name="Heater",device_name="$MAC_DEVICE_1",error="overvoltage",id="2",instance="$INSTANCE_DEVICE_1",mac="$MAC_DEVICE_1"} 0
shelly_status_component_error{component="switch",component_name="Heater",device_name="$MAC_DEVICE_1",error="undervoltage",id="2",instance="$INSTANCE_DEVICE_1",mac="$MAC_DEVICE_1"} 0
shelly_status_component_error{component="switch",component_name="Lift Pump",device_name="$MAC_DEVICE_1",error="overpower",id="1",instance="$INSTANCE_DEVICE_1",mac="$MAC_DEVICE_1"} 0
shelly_status_component_error{component="switch",component_name="Lift Pump",device_name="$MAC_DEVICE_1",error="overtemp",id="1",instance="$INSTANCE_DEVICE_1",mac="$MAC_DEVICE_1"} 0
shelly_status_component_error{component="switch",component_name="Lift Pump",device_name="$MAC_DEVICE_1",error="overvoltage",id="1",instance="$INSTANCE_DEVICE_1",mac="$MAC_DEVICE_1"} 0
shelly_status_component_error{component="switch",component_name="Lift Pump",device_name="$MAC_DEVICE_1",error="undervoltage",id="1",instance="$INSTANCE_DEVICE_1",mac="$MAC_DEVICE_1"} 0
# HELP shelly_status_current_amperes Last measured current in amperes.
# TYPE shelly_status_current_amperes gauge
shelly_status_current_amperes{component="switch",component_name="Bubbles Evac",device_name="$MAC_DEVICE_1",id="3",instance="$INSTANCE_DEVICE_1",mac="$MAC_DEVICE_1"} 0
shelly_status_current_amperes{component="switch",component_name="Change Pump",device_name="$MAC_DEVICE_1",id="0",instance="$INSTANCE_DEVICE_1",mac="$MAC_DEVICE_1"} 0
shelly_status_current_amperes{component="switch",component_name="Heater",device_name="$MAC_DEVICE_1",id="2",instance="$INSTANCE_DEVICE_1",mac="$MAC_DEVICE_1"} 1.728
shelly_status_current_amperes{component="switch",component_name="Lift Pump",device_name="$MAC_DEVICE_1",id="1",instance="$INSTANCE_DEVICE_1",mac="$MAC_DEVICE_1"} 1.135
# HELP shelly_status_input_enabled 1 if the input is enabled; 0 if it is disabled.
# TYPE shelly_status_input_enabled gauge
shelly_status_input_enabled{component_name="input:0",device_name="$MAC_DEVICE_1",id="0",instance="$INSTANCE_DEVICE_1",mac="$MAC_DEVICE_1"} 1
shelly_status_input_enabled{component_name="input:1",device_name="$MAC_DEVICE_1",id="1",instance="$INSTANCE_DEVICE_1",mac="$MAC_DEVICE_1"} 1
shelly_status_input_enabled{component_name="input:2",device_name="$MAC_DEVICE_1",id="2",instance="$INSTANCE_DEVICE_1",mac="$MAC_DEVICE_1"} 1
shelly_status_input_enabled{component_name="input:3",device_name="$MAC_DEVICE_1",id="3",instance="$INSTANCE_DEVICE_1",mac="$MAC_DEVICE_1"} 1
# HELP shelly_status_input_state_on 1 if the input is active; 0 if it is off.
# TYPE shelly_status_input_state_on gauge
shelly_status_input_state_on{component_name="input:0",device_name="$MAC_DEVICE_1",id="0",instance="$INSTANCE_DEVICE_1",mac="$MAC_DEVICE_1"} 0
shelly_status_input_state_on{component_name="input:1",device_name="$MAC_DEVICE_1",id="1",instance="$INSTANCE_DEVICE_1",mac="$MAC_DEVICE_1"} 0
shelly_status_input_state_on{component_name="input:2",device_name="$MAC_DEVICE_1",id="2",instance="$INSTANCE_DEVICE_1",mac="$MAC_DEVICE_1"} 0
shelly_status_input_state_on{component_name="input:3",device_name="$MAC_DEVICE_1",id="3",instance="$INSTANCE_DEVICE_1",mac="$MAC_DEVICE_1"} 0
# HELP shelly_status_instantaneous_active_power_watts Last measured instantaneous active power (in Watts) delivered to the attached load (shown if applicable)
# TYPE shelly_status_instantaneous_active_power_watts gauge
shelly_status_instantaneous_active_power_watts{component="switch",component_name="Bubbles Evac",device_name="$MAC_DEVICE_1",id="3",instance="$INSTANCE_DEVICE_1",mac="$MAC_DEVICE_1"} 0
shelly_status_instantaneous_active_power_watts{component="switch",component_name="Change Pump",device_name="$MAC_DEVICE_1",id="0",instance="$INSTANCE_DEVICE_1",mac="$MAC_DEVICE_1"} 0
shelly_status_instantaneous_active_power_watts{component="switch",component_name="Heater",device_name="$MAC_DEVICE_1",id="2",instance="$INSTANCE_DEVICE_1",mac="$MAC_DEVICE_1"} 206.9
shelly_status_instantaneous_active_power_watts{component="switch",component_name="Lift Pump",device_name="$MAC_DEVICE_1",id="1",instance="$INSTANCE_DEVICE_1",mac="$MAC_DEVICE_1"} 84.4
# HELP shelly_status_network_frequency_hertz Last measured network frequency in Hz.
# TYPE shelly_status_network_frequency_hertz gauge
shelly_status_network_frequency_hertz{component="switch",component_name="Bubbles Evac",device_name="$MAC_DEVICE_1",id="3",instance="$INSTANCE_DEVICE_1",mac="$MAC_DEVICE_1"} 60
shelly_status_network_frequency_hertz{component="switch",component_name="Change Pump",device_name="$MAC_DEVICE_1",id="0",instance="$INSTANCE_DEVICE_1",mac="$MAC_DEVICE_1"} 60
shelly_status_network_frequency_hertz{component="switch",component_name="Heater",device_name="$MAC_DEVICE_1",id="2",instance="$INSTANCE_DEVICE_1",mac="$MAC_DEVICE_1"} 60
shelly_status_network_frequency_hertz{component="switch",component_name="Lift Pump",device_name="$MAC_DEVICE_1",id="1",instance="$INSTANCE_DEVICE_1",mac="$MAC_DEVICE_1"} 60
# HELP shelly_status_power_factor Last measured power factor.
# TYPE shelly_status_power_factor gauge
shelly_status_power_factor{component="switch",component_name="Bubbles Evac",device_name="$MAC_DEVICE_1",id="3",instance="$INSTANCE_DEVICE_1",mac="$MAC_DEVICE_1"} 0
shelly_status_power_factor{component="switch",component_name="Change Pump",device_name="$MAC_DEVICE_1",id="0",instance="$INSTANCE_DEVICE_1",mac="$MAC_DEVICE_1"} 0
shelly_status_power_factor{component="switch",component_name="Heater",device_name="$MAC_DEVICE_1",id="2",instance="$INSTANCE_DEVICE_1",mac="$MAC_DEVICE_1"} 1
shelly_status_power_factor{component="switch",component_name="Lift Pump",device_name="$MAC_DEVICE_1",id="1",instance="$INSTANCE_DEVICE_1",mac="$MAC_DEVICE_1"} 0.62
# HELP shelly_status_switch_output_on 1 if the switch output is on; 0 if it is off.
# TYPE shelly_status_switch_output_on gauge
shelly_status_switch_output_on{component_name="Bubbles Evac",device_name="$MAC_DEVICE_1",id="3",instance="$INSTANCE_DEVICE_1",mac="$MAC_DEVICE_1"} 0
shelly_status_switch_output_on{component_name="Change Pump",device_name="$MAC_DEVICE_1",id="0",instance="$INSTANCE_DEVICE_1",mac="$MAC_DEVICE_1"} 0
shelly_status_switch_output_on{component_name="Heater",device_name="$MAC_DEVICE_1",id="2",instance="$INSTANCE_DEVICE_1",mac="$MAC_DEVICE_1"} 1
shelly_status_switch_output_on{component_name="Lift Pump",device_name="$MAC_DEVICE_1",id="1",instance="$INSTANCE_DEVICE_1",mac="$MAC_DEVICE_1"} 1
# HELP shelly_status_temperature_celsius Temperature in degrees celsius.
# TYPE shelly_status_temperature_celsius gauge
shelly_status_temperature_celsius{component="switch",component_name="Bubbles Evac",device_name="$MAC_DEVICE_1",id="3",instance="$INSTANCE_DEVICE_1",mac="$MAC_DEVICE_1"} 44.4
shelly_status_temperature_celsius{component="switch",component_name="Change Pump",device_name="$MAC_DEVICE_1",id="0",instance="$INSTANCE_DEVICE_1",mac="$MAC_DEVICE_1"} 44.4
shelly_status_temperature_celsius{component="switch",component_name="Heater",device_name="$MAC_DEVICE_1",id="2",instance="$INSTANCE_DEVICE_1",mac="$MAC_DEVICE_1"} 44.4
shelly_status_temperature_celsius{component="switch",component_name="Lift Pump",device_name="$MAC_DEVICE_1",id="1",instance="$INSTANCE_DEVICE_1",mac="$MAC_DEVICE_1"} 44.4
# HELP shelly_status_temperature_fahrenheit Temperature in degrees farenheit.
# TYPE shelly_status_temperature_fahrenheit gauge
shelly_status_temperature_fahrenheit{component="switch",component_name="Bubbles Evac",device_name="$MAC_DEVICE_1",id="3",instance="$INSTANCE_DEVICE_1",mac="$MAC_DEVICE_1"} 111.9
shelly_status_temperature_fahrenheit{component="switch",component_name="Change Pump",device_name="$MAC_DEVICE_1",id="0",instance="$INSTANCE_DEVICE_1",mac="$MAC_DEVICE_1"} 111.9
shelly_status_temperature_fahrenheit{component="switch",component_name="Heater",device_name="$MAC_DEVICE_1",id="2",instance="$INSTANCE_DEVICE_1",mac="$MAC_DEVICE_1"} 111.9
shelly_status_temperature_fahrenheit{component="switch",component_name="Lift Pump",device_name="$MAC_DEVICE_1",id="1",instance="$INSTANCE_DEVICE_1",mac="$MAC_DEVICE_1"} 111.9
# HELP shelly_status_total_energy_watt_hours Total energy consumed in Watt-hours.
# TYPE shelly_status_total_energy_watt_hours counter
shelly_status_total_energy_watt_hours{component="switch",component_name="Bubbles Evac",device_name="$MAC_DEVICE_1",id="3",instance="$INSTANCE_DEVICE_1",mac="$MAC_DEVICE_1"} 16.973
shelly_status_total_energy_watt_hours{component="switch",component_name="Change Pump",device_name="$MAC_DEVICE_1",id="0",instance="$INSTANCE_DEVICE_1",mac="$MAC_DEVICE_1"} 1759.789
shelly_status_total_energy_watt_hours{component="switch",component_name="Heater",device_name="$MAC_DEVICE_1",id="2",instance="$INSTANCE_DEVICE_1",mac="$MAC_DEVICE_1"} 97495.339
shelly_status_total_energy_watt_hours{component="switch",component_name="Lift Pump",device_name="$MAC_DEVICE_1",id="1",instance="$INSTANCE_DEVICE_1",mac="$MAC_DEVICE_1"} 121397.383
# HELP shelly_status_total_returned_energy_watt_hours Total returned energy consumed in Watt-hours
# TYPE shelly_status_total_returned_energy_watt_hours counter
shelly_status_total_returned_energy_watt_hours{component="switch",component_name="Bubbles Evac",device_name="$MAC_DEVICE_1",id="3",instance="$INSTANCE_DEVICE_1",mac="$MAC_DEVICE_1"} 0
shelly_status_total_returned_energy_watt_hours{component="switch",component_name="Change Pump",device_name="$MAC_DEVICE_1",id="0",instance="$INSTANCE_DEVICE_1",mac="$MAC_DEVICE_1"} 0
shelly_status_total_returned_energy_watt_hours{component="switch",component_name="Heater",device_name="$MAC_DEVICE_1",id="2",instance="$INSTANCE_DEVICE_1",mac="$MAC_DEVICE_1"} 0
shelly_status_total_returned_energy_watt_hours{component="switch",component_name="Lift Pump",device_name="$MAC_DEVICE_1",id="1",instance="$INSTANCE_DEVICE_1",mac="$MAC_DEVICE_1"} 0
# HELP shelly_status_voltage Last measured voltage.
# TYPE shelly_status_voltage gauge
shelly_status_voltage{component="switch",component_name="Bubbles Evac",device_name="$MAC_DEVICE_1",id="3",instance="$INSTANCE_DEVICE_1",mac="$MAC_DEVICE_1"} 119.8
shelly_status_voltage{component="switch",component_name="Change Pump",device_name="$MAC_DEVICE_1",id="0",instance="$INSTANCE_DEVICE_1",mac="$MAC_DEVICE_1"} 119.8
shelly_status_voltage{component="switch",component_name="Heater",device_name="$MAC_DEVICE_1",id="2",instance="$INSTANCE_DEVICE_1",mac="$MAC_DEVICE_1"} 119.8
shelly_status_voltage{component="switch",component_name="Lift Pump",device_name="$MAC_DEVICE_1",id="1",instance="$INSTANCE_DEVICE_1",mac="$MAC_DEVICE_1"} 119.8
`,
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			td := discovery.NewTestDiscoverer(t)
			d1 := td.NewTestDevice(t, true)
			d1.AddMockResponse("Shelly.GetStatus", nil, json.RawMessage(tc.d1Status))
			d1.AddMockResponse("Shelly.GetConfig", nil, json.RawMessage(tc.d1Config))

			_, ps := NewServer(ctx, td.Discoverer)
			metricserver := httptest.NewServer(ps)
			t.Cleanup(metricserver.Close)

			fakeResp, err := http.Get(metricserver.URL)
			require.NoError(t, err)
			body, err := io.ReadAll(fakeResp.Body)
			require.NoError(t, err)
			t.Log(string(body))

			expect := strings.NewReplacer("$INSTANCE_DEVICE_1", d1.Instance(), "$MAC_DEVICE_1", d1.MACAddr).Replace(tc.expect)
			require.NoError(t, testutil.ScrapeAndCompare(metricserver.URL, bytes.NewBufferString(expect)))
		})
	}
}
