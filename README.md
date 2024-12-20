# shellyctl
shellyctl is an unofficial command line client for the [Shelly Gen2 API](https://shelly-api-docs.shelly.cloud/gen2/).

## Features
* mDNS discovery of shelly devices on the local network.
* Bluetooth Low Energy (BLE) discovery of shelly devices for RPC, monitoring, and initial setup.
* Command line interface for documented APIs.
* prometheus metrics endpoint with the status of known devices.

## Maturity
This library is currently in active development (as of February 2024). It has meaningful gaps in testing and functionality. At this stage there is no guarantee of backwards compatibility. 

## Usage
```
shellyctl provides a cli interface for discovering and working with shelly gen 2 devices

Usage:
  shellyctl [flags]
  shellyctl [command]

Device Component RPCs:
  ble         RPCs related to Bluetooth Low-Energy
  cloud       RPCs related to Shelly Cloud
  cover       RPCs related to Cover components
  input       RPCs related to input components
  light       RPCs related to light components
  mqtt        RPCs related to MQTT configuration and status
  schedule    RPCs related to managing schedules
  shelly      RPCs related device management, configuration, and status
  switch      RPCs related to switch components
  sys         RPCs related to system management and status
  wifi        RPCs related to wifi configuration and status.

Servers:
  prometheus  Host a prometheus metrics exporter for shelly devices

Additional Commands:
  completion  Generate the autocompletion script for the specified shell
  help        Help about any command

Flags:
      --config string          path to config file. format will be determined by extension (.yaml, .json, .toml, .ini valid)
      --log-level string       threshold for outputing logs: trace, debug, info, warn, error, fatal, panic (default "warn")
  -o, --output-format string   desired output format: json, min-json, yaml, text, log (default "text")

Use "shellyctl [command] --help" for more information about a command.
```

### Configuration
This utility supports configuration via command-line flags, environment variables, or a config file.

#### Environment Variable Configuration
Command flags (documented as `--flag-name`) may alternatively be set as environment variables namespaced with the `SHELLYCTL_` prefix and with underscores `_` replacing `-` hyphens. For example, if you're uncomfortable passing the authentication password as a command-line argument, you could pass it as an environment variable. 

The following two examples have identical functionality.
```
shellyctl shelly get-status --auth=my-secret-password --log-level=debug --mdns-search

SHELLYCTL_AUTH=my-secret-password SHELLYCTL_LOG_LEVEL=debug SHELLYCTL_MDNS_SEARCH=true shellyctl shelly get-status 
```
#### Config File
Command flags (documented as `--flag-name`) may be listed in a YAML (extension .yaml), JSON (.json), TOML (.toml), or INI (.ini) file.

For example:
```yaml
bind-port: 19090
log-level: debug
host:
- http://192.168.1.1/
- http://admin:password@192.168.1.2/
```

Could be used as a config file for the `shellyctl prometheus` command.
```
shellyctl prometheus --config=config.yaml
```

### Prometheus Server
The `prometheus` sub-command facilitates running a [Prometheus](https://prometheus.io/) compatible metrics server.

See [contrib/k8s](contrib/k8s) for instructions on running the prometheus server within a Kubernetes cluster.

```
Host a prometheus metrics exporter for shelly devices

Usage:
  shellyctl prometheus [flags]

Aliases:
  prometheus, prom

Flags:
      --auth string                        password to use for authenticating with devices.
      --bind-addr ip                       local ip address to bind the metrics server to (default ::)
      --bind-port uint16                   port to bind the metrics server (default 8080)
      --ble-device stringArray             MAC address of a single bluetooth low-energy device. May be specified multiple times to work with multiple devices.
      --ble-search                         if true, devices will be discovered via Bluetooth Low-Energy
      --device-timeout duration            set the maximum time allowed for a device to respond to it probe. (default 5s)
      --device-ttl duration                time-to-live for discovered devices in long-lived commands like the prometheus server. (default 5m0s)
      --discovery-concurrency int          number of concurrent  (default 5)
  -h, --help                               help for prometheus
      --host http                          host address of a single device. IP, DNS, or mDNS/BonJour addresses are accepted.
                                           If a URL scheme is provided, only http and `https` schemes are supported.
                                           mDNS names must be within the zone specified by the `--mdns-zone` flag (default `local`).
                                           URL formatted auth is supported (ex. `http://admin:password@1.2.3.4/`)
      --interactive                        if true prompt for confirmation or passwords.
      --mdns-interface string              if specified, search only the specified network interface for devices.
      --mdns-search                        if true, devices will be discovered via mDNS
      --mdns-service string                mDNS service to search (default "_shelly._tcp")
      --mdns-zone string                   mDNS zone to search (default "local")
      --prefer-ip-version 4                prefer ip version (4 or `6`)
      --probe-concurrency int              set the number of concurrent probes which will be made to service a metrics request. (default 10)
      --prometheus-namespace string        set the namespace string to use for prometheus metric names. (default "shelly")
      --prometheus-subsystem string        set the subsystem section of the prometheus metric names. (default "status")
      --scrape-duration-warning duration   sets the value for scrape duration warning. Scrapes which exceed this duration will log a warning generate. Default value 8s is 80% of the 10s default prometheus scrape_timeout. (default 8s)
      --search-interactive                 if true confirm devices discovered in search before proceeding with commands. Defers to --interactive if not explicitly set.
      --search-strict-timeout              ignore devices which have been found but completed their initial query within the search-timeout (default true)
      --search-timeout duration            timeout for devices to respond to the mDNS discovery query. (default 1s)
      --skip-failed-hosts                  continue with other hosts in the face errors.

Global Flags:
      --config string          path to config file. format will be determined by extension (.yaml, .json, .toml, .ini valid)
      --log-level string       threshold for outputing logs: trace, debug, info, warn, error, fatal, panic (default "warn")
  -o, --output-format string   desired output format: json, min-json, yaml, text, log (default "text")
```

### RPC Command-line

#### Example
```
$ shellyctl switch set-config --ble-device=AA:BB:CC:DD:EE:FF --auto-off=true --auto-off-delay=60

Response to Switch.SetConfig command for ShellyPlugUS-AABBCCDDEEFF:
  Restart Required: false

$ shellyctl switch set --ble-device=AA:BB:CC:DD:EE:FF --on=true

Response to Switch.Set command for ShellyPlugUS-AABBCCDDEEFF:
  Was On: true
```

#### Updating Devices
Shelly devices support self-updates via the ([Shelly.Update](https://shelly-api-docs.shelly.cloud/gen2/ComponentsAndServices/Shelly#shellyupdate)) RPC.  This can be invoked on the command-line with `shellyctl shelly update`. Individual devices can be specified with the `--host` flag, or in bulk using mDNS (`--mdns-search`) or BLE (`--ble-search`) discovery. If no updates are available for the specified release stage, the Shelly devices appear to return error `-114: Resource unavailable: No update info!`; consider using the `--skip-failed-hosts` flag to continue in the face of this error.
```
shellyctl shelly update --stage=stable --skip-failed-hosts=true --mdns-search --interactive=false

Response to Shelly.Update command for http://192.168.1.28/rpc: success

7:31PM ERR error executing request; contining because --skip-failed-hosts=true error="RPC Bad Status -114: Resource unavailable: No update info!" component=discovery
 device_name=ShellyPro3-AABBCCDDEEFF instance=http://192.168.1.12:80/rpc request=Shelly.Update

Response to Shelly.Update command for http://192.168.1.29/rpc: success
 ```

#### Menu Heirarchy
- `ble`
  - `get-config` ([BLE.GetConfig](https://shelly-api-docs.shelly.cloud/gen2/ComponentsAndServices/BLE/#blegetconfig))
  - `get-status` ([BLE.GetStatus](https://shelly-api-docs.shelly.cloud/gen2/ComponentsAndServices/BLE/#blegetstatus))
  - `set-config` ([BLE.SetConfig](https://shelly-api-docs.shelly.cloud/gen2/ComponentsAndServices/BLE/#blesetconfig))
- `cloud`
  - `get-config` ([Cloud.GetConfig](https://shelly-api-docs.shelly.cloud/gen2/ComponentsAndServices/Cloud/#cloudgetconfig))
  - `get-status` ([Cloud.GetStatus](https://shelly-api-docs.shelly.cloud/gen2/ComponentsAndServices/Cloud/#cloudgetstatus))
  - `set-config` ([Cloud.SetConfig](https://shelly-api-docs.shelly.cloud/gen2/ComponentsAndServices/Cloud/#cloudsetconfig))
- `cover`
  - `calibrate` ([Cover.Calibrate](https://shelly-api-docs.shelly.cloud/gen2/ComponentsAndServices/Cover/#covercalibrate))
  - `close` ([Cover.Close](https://shelly-api-docs.shelly.cloud/gen2/ComponentsAndServices/Cover/#coverclose))
  - `get-config` ([Cover.GetConfig](https://shelly-api-docs.shelly.cloud/gen2/ComponentsAndServices/Cover/#covergetconfig))
  - `get-status` ([Cover.GetStatus](https://shelly-api-docs.shelly.cloud/gen2/ComponentsAndServices/Cover/#covergetstatus))
  - `go-to-position` ([Cover.GoToPosition](https://shelly-api-docs.shelly.cloud/gen2/ComponentsAndServices/Cover/#covergotoposition))
  - `open` ([Cover.Open](https://shelly-api-docs.shelly.cloud/gen2/ComponentsAndServices/Cover/#coveropen))
  - `reset-counters` ([Cover.ResetCounters](https://shelly-api-docs.shelly.cloud/gen2/ComponentsAndServices/Cover/#coverresetcounters))
  - `set-config` ([Cover.SetConfig](https://shelly-api-docs.shelly.cloud/gen2/ComponentsAndServices/Cover/#coversetconfig))
  - `stop` ([Cover.Stop](https://shelly-api-docs.shelly.cloud/gen2/ComponentsAndServices/Cover/#coverstop))
- `input`
  - `check-expression` ([Input.GetConfig](https://shelly-api-docs.shelly.cloud/gen2/ComponentsAndServices/Input#inputcheckexpression))
  - `get-config` ([Input.GetConfig](https://shelly-api-docs.shelly.cloud/gen2/ComponentsAndServices/Input#inputsetconfig))
  - `get-status` ([Input.GetStatus](https://shelly-api-docs.shelly.cloud/gen2/ComponentsAndServices/Input#inputgetstatus))
  - `set-config` ([Input.SetConfig](https://shelly-api-docs.shelly.cloud/gen2/ComponentsAndServices/Input#inputsetconfig))
- `light`
  - `get-config` ([Light.GetConfig](https://shelly-api-docs.shelly.cloud/gen2/ComponentsAndServices/Light#lightgetconfig))
  - `get-status` ([Light.GetStatus](https://shelly-api-docs.shelly.cloud/gen2/ComponentsAndServices/Light#lightgetstatus))
  - `set` ([Light.Set](https://shelly-api-docs.shelly.cloud/gen2/ComponentsAndServices/Light#lightset))
  - `set-config` ([Light.SetConfig](https://shelly-api-docs.shelly.cloud/gen2/ComponentsAndServices/Light#lightsetconfig))
  - `toggle` ([Light.Toggle](https://shelly-api-docs.shelly.cloud/gen2/ComponentsAndServices/Light#lighttoggle))
- `mqtt`
  - `get-config` ([MQTT.GetConfig](https://shelly-api-docs.shelly.cloud/gen2/ComponentsAndServices/Mqtt#mqttgetconfig))
  - `get-status` ([MQTT.GetStatus](https://shelly-api-docs.shelly.cloud/gen2/ComponentsAndServices/Mqtt#mqttgetstatus))
  - `set-config` ([MQTT.SetConfig](https://shelly-api-docs.shelly.cloud/gen2/ComponentsAndServices/Mqtt#mqttsetconfig))
- `schedule`
  - `delete` ([Schedule.Delete](https://shelly-api-docs.shelly.cloud/gen2/ComponentsAndServices/Schedule#scheduledelete))
  - `delete-all` ([Schedule.DeleteAll](https://shelly-api-docs.shelly.cloud/gen2/ComponentsAndServices/Schedule#scheduledeleteall))
- `script`
  - `create` ([Script.Create](https://shelly-api-docs.shelly.cloud/gen2/ComponentsAndServices/Script#scriptcreate))
  - `delete` ([Script.Delete](https://shelly-api-docs.shelly.cloud/gen2/ComponentsAndServices/Script#scriptdelete))
  - `eval` ([Script.Eval](https://shelly-api-docs.shelly.cloud/gen2/ComponentsAndServices/Script#scripteval))
  - `get-config` ([Script.GetConfig](https://shelly-api-docs.shelly.cloud/gen2/ComponentsAndServices/Script#scriptgetconfig))
  - `get-status` ([Script.GetStatus](https://shelly-api-docs.shelly.cloud/gen2/ComponentsAndServices/Script#scriptgetstatus))
  - `list` ([Script.List](https://shelly-api-docs.shelly.cloud/gen2/ComponentsAndServices/Script#scriptlist))
  - `put-code` ([Script.PutCode](https://shelly-api-docs.shelly.cloud/gen2/ComponentsAndServices/Script#scriptputcode))
  - `set-config` ([Script.SetConfig](https://shelly-api-docs.shelly.cloud/gen2/ComponentsAndServices/Script#scriptsetconfig))
  - `start` ([Script.Start](https://shelly-api-docs.shelly.cloud/gen2/ComponentsAndServices/Script#scriptstart))
  - `stop` ([Script.Stop](https://shelly-api-docs.shelly.cloud/gen2/ComponentsAndServices/Script#scriptstop))
- `shelly`
  - `check-for-update` ([Shelly.CheckForUpdate](https://shelly-api-docs.shelly.cloud/gen2/ComponentsAndServices/Shelly#shellycheckforupdate))
  - `detect-location` ([Shelly.DetectLocation](https://shelly-api-docs.shelly.cloud/gen2/ComponentsAndServices/Shelly#shellydetectlocation))
  - `factory-reset` ([Shelly.FactoryReset](https://shelly-api-docs.shelly.cloud/gen2/ComponentsAndServices/Shelly#shellyfactoryreset))
  - `get-config` ([Shelly.GetConfig](https://shelly-api-docs.shelly.cloud/gen2/ComponentsAndServices/Shelly#shellygetconfig))
  - `get-status` ([Shelly.GetStatus](https://shelly-api-docs.shelly.cloud/gen2/ComponentsAndServices/Shelly#shellygetstatus))
  - `list-methods` ([Shelly.ListMethods](https://shelly-api-docs.shelly.cloud/gen2/ComponentsAndServices/Shelly#shellylistmethods))
  - `list-profiles` ([Shelly.ListProfiles](https://shelly-api-docs.shelly.cloud/gen2/ComponentsAndServices/Shelly#shellylistprofiles))
  - `list-timezones` ([Shelly.ListTimezones](https://shelly-api-docs.shelly.cloud/gen2/ComponentsAndServices/Shelly#shellylisttimezones))
  - `put-tls-client-cert` ([Shelly.PutTLSClientCert](https://shelly-api-docs.shelly.cloud/gen2/ComponentsAndServices/Shelly#shellyputtlsclientcert))
  - `put-tls-client-key` ([Shelly.PutTLSClientKey](https://shelly-api-docs.shelly.cloud/gen2/ComponentsAndServices/Shelly#shellyputtlsclientkey))
  - `put-user-ca` ([Shelly.PutUserCA](https://shelly-api-docs.shelly.cloud/gen2/ComponentsAndServices/Shelly#shellyputuserca))
  - `reboot` ([Shelly.Reboot](https://shelly-api-docs.shelly.cloud/gen2/ComponentsAndServices/Shelly#shellyreboot))
  - `reset-wifi-config` ([Shelly.ResetWiFiConfig](https://shelly-api-docs.shelly.cloud/gen2/ComponentsAndServices/Shelly#shellyresetwificonfig))
  - `set-auth` ([Shelly.SetAuth](https://shelly-api-docs.shelly.cloud/gen2/ComponentsAndServices/Shelly#shellysetauth))
  - `set-profile` ([Shelly.SetProfile](https://shelly-api-docs.shelly.cloud/gen2/ComponentsAndServices/Shelly#shellysetprofile))
  - `update` ([Shelly.Update](https://shelly-api-docs.shelly.cloud/gen2/ComponentsAndServices/Shelly#shellyupdate))
- `switch`
  - `get-config` ([Switch.GetConfig](https://shelly-api-docs.shelly.cloud/gen2/ComponentsAndServices/Switch#switchgetconfig))
  - `get-status` ([Switch.GetStatus](https://shelly-api-docs.shelly.cloud/gen2/ComponentsAndServices/Switch#switchgetstatusg))
  - `set` ([Switch.Set](https://shelly-api-docs.shelly.cloud/gen2/ComponentsAndServices/Switch#switchset))
  - `set-config` ([Switch.SetConfig](https://shelly-api-docs.shelly.cloud/gen2/ComponentsAndServices/Switch#switchsetconfig))
  - `toggle` ([Switch.Toggle](https://shelly-api-docs.shelly.cloud/gen2/ComponentsAndServices/Switch#switchtoggle))
- `sys`
  - `get-config` ([Sys.GetConfig](https://shelly-api-docs.shelly.cloud/gen2/ComponentsAndServices/Sys#sysgetconfig))
  - `get-status` ([Sys.GetStatus](https://shelly-api-docs.shelly.cloud/gen2/ComponentsAndServices/Sys#sysgetstatus))
  - `set-config` ([Sys.SetConfig](https://shelly-api-docs.shelly.cloud/gen2/ComponentsAndServices/Sys#syssetconfig))
- `wifi`
  - `get-config` ([WiFi.GetConfig](https://shelly-api-docs.shelly.cloud/gen2/ComponentsAndServices/WiFi#wifigetconfig))
  - `get-status` ([WiFi.GetStatus](https://shelly-api-docs.shelly.cloud/gen2/ComponentsAndServices/WiFi#wifigetstatus))
  - `set-config` ([WiFi.SetConfig](https://shelly-api-docs.shelly.cloud/gen2/ComponentsAndServices/WiFi#wifisetconfig))


### Device Initial Setup
By default Shelly devices can be configured with RPCs over Bluetooth Low Energy (BLE) channel. The initial configuration is therefore just a matter of configuring network connectivity, optionally disabling BLE, and optionally setting authentication.
```
$ shellyctl wifi set-config --sta-enable=true --sta-ssid=INTERNET --sta-pass=password --ble-search

Found BLE device "ShellyPlugUS-AABBCCDDEEFF" (AA:BB:CC:DD:EE:FF)
y - Add device and continue search
n - Skip this device and continue search
u - Use this device and stop searching for additional devices
a - Abort search without this device
q - Quit without acting on this device or any others
Use this device [y,n,u,a,q]?
y
Response to Wifi.SetConfig command for ShellyPlugUS-AABBCCDDEEFF:
  Restart Required: false

$ shellyctl ble set-config --enable=false --host=192.168.1.62

Response to BLE.SetConfig command for ShellyPlugUS-AABBCCDDEEFF:
  Restart Required: true

$ shellyctl shelly reboot --host=192.168.1.62

Response to Shelly.Reboot command for ShellyPlugUS-AABBCCDDEEFF:
```

## TODO
* Device Backup & Restore / Support for configuration as code style provisioning.
* WebSocket support
* Support for shelly debug logs via Websockets, MQTT, or UDP.
* Missing Methods:
  * Script.GetCode


## Releases
See: [Releases](https://github.com/jcodybaker/shellyctl/releases)

Additionally container images are available: See [GitHub Packages](https://github.com/jcodybaker/shellyctl/pkgs/container/shellyctl/versions?filters%5Bversion_type%5D=tagged)

## Contributing
Pull-requests and [issues](https://github.com/jcodybaker/go-shelly/issues) are welcome. Code should be formatted with gofmt, pass existing tests, and ideally add new testing. Test should include samples from live device request/response flows when possible.

## Legal

### Intellectual Property
This utility and its authors (Cody Baker - cody@codybaker.com) have no affiliation with [Allterco Robotics](https://allterco.com/) (the maker of [Shelly](https://www.shelly.com/) devices) or [Cesanta Software Limited](https://cesanta.com/) (the maker of [MongooseOS](https://mongoose-os.com/)).  All trademarks are property of their respective owners.  Importantly, the "[Shelly](https://www.shelly.com/)" name and names of devices are trademarks of [Allterco Robotics](https://allterco.com/). [MongooseOS](https://mongoose-os.com/) and related trademarks are property of [Cesanta Software Limited](https://cesanta.com/).

### Liability
By downloading/using this open-source utilty you indemnify the authors of this project ([J Cody Baker](cody@codybaker.com)) from liability to the fullest extent permitted by law. There are real risks of fire, electricution, and/or bodily injury/death with these devices and connected equipment. Errors, bugs, or misinformation may exist in this software which cause your device and attached equipment to function in unexpected and dangerous ways. That risk is your responsibility. 

### License 
Copyright 2023 [J Cody Baker](cody@codybaker.com) and licensed under the [MIT license](LICENSE.md).