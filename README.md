# shellyctl
shellyctl is an unofficial command line client for the [Shelly Gen2 API](https://shelly-api-docs.shelly.cloud/gen2/).

## Features
* mDNS discovery of shelly devices on the local network.
* prometheus metrics endpoint with the status of known devices.

## Maturity
This library is currently in active development (as of December 2023). It has meaningful gaps in testing and functionality. At this stage there is no guarantee of backwards compatibility. Once the project reaches a stable state, I will begin crafting releases with semantic versioning. 

## Usage
```
shellyctl provides a cli interface for discovering and working with shelly gen 2 devices

Usage:
  shellyctl [flags]
  shellyctl [command]

Available Commands:
  completion  Generate the autocompletion script for the specified shell
  help        Help about any command
  prometheus  host a prometheus metrics exporter for shelly devices

Flags:
  -h, --help               help for shellyctl
      --log-level string   threshold for outputing logs: trace, debug, info, warn, error, fatal, panic (default "warn")

Use "shellyctl [command] --help" for more information about a command.
```

### Prometheus Server
```
host a prometheus metrics exporter for shelly devices

Usage:
  shellyctl prometheus [flags]

Aliases:
  prometheus, prom

Flags:
      --bind-addr ip                  local ip address to bind the metrics server to (default ::)
      --bind-port uint16              port to bind the metrics server (default 8080)
      --device-timeout duration       set the maximum time allowed for a device to respond to it probe. (default 5s)
      --device-ttl duration           time-to-live for discovered devices in long-lived commands like the prometheus server. (default 5m0s)
      --discovery-concurrency int     number of concurrent  (default 5)
  -h, --help                          help for prometheus
      --host http                     host address of a single device. IP, DNS, or mDNS/BonJour addresses are accepted. If a URL scheme is provided, only http and `https` are supported. mDNS names must be within the zone specified by the `--mdns-zone` flag (default `local`).
      --mdns-interface string         if specified, search only the specified network interface for devices.
      --mdns-search                   if true, devices will be discovered via mDNS
      --mdns-service string           mDNS service to search (default "_shelly._tcp")
      --mdns-zone string              mDNS zone to search (default "local")
      --prefer-ip-version 4           prefer ip version (4 or `6`)
      --probe-concurrency int         set the number of concurrent probes which will be made to service a metrics request. (default 10)
      --prometheus-namespace string   set the namespace string to use for prometheus metric names. (default "shelly")
      --prometheus-subsystem string   set the subsystem section of the prometheus metric names. (default "status")
      --search-timeout duration       timeout for devices to respond to the mDNS discovery query. (default 1s)
      --skip-failed-hosts             continue with other hosts in the face errors.

Global Flags:
      --log-level string   threshold for outputing logs: trace, debug, info, warn, error, fatal, panic (default "warn")
```

## TODO
* BLE Support
* Device Provisioning
* Device Backup & Restore / Support for configuration as code style provisioning.
* CLI Support for all configuration
* MQTT / WebSocket support


## Contributing
Pull-requests and [issues](https://github.com/jcodybaker/go-shelly/issues) are welcome. Code should be formatted with gofmt, pass existing tests, and ideally add new testing. Test should include samples from live device request/response flows when possible.

## Legal

### Intellectual Property
This utility and its authors (Cody Baker - cody@codybaker.com) have no affiliation with [Allterco Robotics](https://allterco.com/) (the maker of [Shelly](https://www.shelly.com/) devices) or [Cesanta Software Limited](https://cesanta.com/) (the maker of [MongooseOS](https://mongoose-os.com/)).  All trademarks are property of their respective owners.  Importantly, the "[Shelly](https://www.shelly.com/)" name and names of devices are trademarks of [Allterco Robotics](https://allterco.com/). [MongooseOS](https://mongoose-os.com/) and related trademarks are property of [Cesanta Software Limited](https://cesanta.com/).

### Liability
By downloading/using this open-source utilty you indemnify the authors of this project ([J Cody Baker](cody@codybaker.com)) from liability to the fullest extent permitted by law. There are real risks of fire, electricution, and/or bodily injury/death with these devices and connected equipment. Errors, bugs, or misinformation may exist in this software which cause your device and attached equipment to function in unexpected and dangerous ways. That risk is your responsibility. 

### License 
Copyright 2023 [J Cody Baker](cody@codybaker.com) and licensed under the [MIT license](LICENSE.md).