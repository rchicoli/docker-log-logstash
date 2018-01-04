# Docker Log Elasticsearch

`docker-log-logstash` forwards container logs a TCP server (e.g. Logstash with the TCP input plugin).

This application is under active development and will continue to be modified and improved over time. The current release is an "alpha".

## Releases

| Branch Name | Docker Tag | Logstash Version | Remark |
| ----------- | ---------- | --------------------- | ------ |
| release-1.5.x  | 1.5.x   | 5.x                | Future stable release. |
| alpha-0.5.x    | 0.5.1, 0.5.2   | 5.x                | Actively alpha release. |

```
release-0.5.1
        | | |_ new features or bug fixes
        | |___ logstash major version
        |_____ release version
```

## Getting Started

You need to install Docker Engine >= 1.12 and Logstash 5

Additional information about Docker plugins [can be found here](https://docs.docker.com/engine/extend/plugins_logging/).

### Installing

To install the plugin, run

    docker plugin install rchicoli/docker-log-logstash:0.5.1 --alias logstash

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

#### Testing

Creating and running a container:

    $ docker run --rm  -ti \
        --log-driver logstash \
        --log-opt logstash-url=tcp://127.0.0.1:5000 \
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
| @timestamp | Timestamp that the log was collected by logstash | yes |
| partial | Whether docker reported that the log message was only partially collected | no |
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
| err | Usually null, otherwise will be a string containing and error from the logdriver | yes |