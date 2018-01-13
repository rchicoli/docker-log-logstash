# Docker Log Logstash

`docker-log-logstash` forwards container logs a TCP/UDP server  (e.g. Logstash with the TCP input plugin) or even to a socket.

This application is under active development and will continue to be modified and improved over time. The current release is an "alpha".

## Goal

The goal of this project is to create a reliable log plugin for docker.
It basically sends alls logs to Logstash, if this service becomes unavailable, all messages should be written to a logfile and send them later to Logstash, when the service is responding again.

## Development Status

This plugin is capable of:
  * reconnecting to logstash server, in case of lost connection (at the moment, only if logstash is using the host network)
  * caching messages to the filesystem, while logstash is down
  * send cached log information to logstash, when it is online

## Releases

| Branch Name | Docker Tag | Remark |
| ----------- | ---------- | ------ |
| alpha-0.0.x    | 0.0.1, 0.0.2   | Actively alpha release. |

## Getting Started

You need to install Docker Engine >= 1.12 and Logstash

Additional information about Docker plugins [can be found here](https://docs.docker.com/engine/extend/plugins_logging/).

### Installing

To install the plugin, run

    docker plugin install rchicoli/docker-log-logstash:0.0.6 --alias logstash

This command will pull and enable the plugin

### Using

First of all, it is required to have Logstash up and running. In your logstash config, set the input codec to json e.g:

```
input {
  udp {
    port  => 5000
    codec => json
  }
  tcp {
    port  => 5000
    codec => json
  }
}
```

#### Note

To run a specific container with the logging driver:

    Use the --log-driver flag to specify the plugin.
    Use the --log-opt flag to specify the URL for the HTTP connection and further options.

**Options**

| Key | Default Value | Required | Examples |
| --- | ------------- | -------- | ------- |
| logstash-url   | no     | yes | tcp://127.0.0.1:5000, udp://127.0.0.1:5000 |
| logstash-timeout | 1000ms | no | 1, 10, 1000 in ms |
| logstash-fields | containerID,containerName,containerImageName,containerCreated | no | containerID,containerLabels,containerEnv |

#### Testing

Creating and running a container:

    $ docker run --rm  -ti \
        --log-driver logstash \
        --log-opt logstash-url=tcp://127.0.0.1:5000 \
        --log-opt logstash-fields=containerID,containerName,containerImageName,containerCreated,logPath
            alpine echo this is another logging driver

## Output Format

By using `rubydebug` as stdout codec:

```bash
{
            "@timestamp" => 2018-01-03T21:15:13.481Z,
                  "port" => 53728,
         "containerName" => "focused_lumiere",
    "containerImageName" => "alpine",
      "containerCreated" => "2018-01-03T21:15:13.080275904Z",
              "@version" => "1",
                  "host" => "127.0.0.1",
                "source" => "stdout",
           "containerID" => "ba5dabbff0de",
               "message" => "this is another logging driver"
}
```

**Fields**

| Field | Description | Default |
| ----- | ----------- | ------- |
| message  | The log message itself | yes |
| source | Source of the log message as reported by docker | yes |
| @timestamp | Timestamp that the log was collected by logstash | yes (by logstash) |
| timestamp | Timestamp from the container's log | yes |
| partial | Whether docker reported that the log message was only partially collected | yes |
| containerID | Id of the container that generated the log message | yes |
| containerName | Name of the container that generated the log message | yes |
| containerArgs | Arguments of the container entrypoint | no |
| containerImageID | ID of the container's image | no |
| containerImageName | Name of the container's image | yes |
| containerCreated | Timestamp of the container's creation | yes |
| containerEnv | Environment of the container | no |
| containerLabels | Label of the container | no |
| containerLogPath | Path of the container's Log | no |
| daemonName | Name of the container's daemon | no |
| err | Usually null, otherwise will be a string containing and error from the logdriver | no |