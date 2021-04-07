# Chaos Master

[![Build Status](https://travis-ci.org/SotirisAlfonsos/chaos-master.svg)](https://travis-ci.org/SotirisAlfonsos/chaos-master)
[![Go Report Card](https://goreportcard.com/badge/github.com/SotirisAlfonsos/chaos-master)](https://goreportcard.com/report/github.com/SotirisAlfonsos/chaos-master)
[![codebeat badge](https://codebeat.co/badges/ab1778ae-60c1-4b7d-aff6-a8f1eabbd2d5)](https://codebeat.co/projects/github-com-sotirisalfonsos-chaos-master-master)
[![codecov.io](https://codecov.io/github/SotirisAlfonsos/chaos-master/coverage.svg?branch=master)](https://codecov.io/github/SotirisAlfonsos/chaos-master?branch=master)

The master provides an api to orchestrate fault injections to the [chaos bots](https://github.com/SotirisAlfonsos/chaos-bot). Using the chaos API you have access to a number of possible fault injection and to an automatic failure recovery mechanism
#### [Chaos in practice](https://principlesofchaos.org/)
1. Start by defining a ‘steady state’.
2. Hypothesize that this steady state will continue in both the control group and the experimental group.
3. <b>Inject failures that reflect real world events.</b>
4. <i>Try to disprove the hypothesis by looking for a difference in steady state between the control group and the experimental group.</i>  

At this point we should add one more stage

5. <b>Recover fast to the ‘steady state’.</b>

The master and bots focus on two of the stages of chaos, <b>Injection of failures</b> and <b>recovery to a steady state</b>

----

## Starting Up

#### Create your project flder and download the latest chaos master binary

```bash
wget https://github.com/SotirisAlfonsos/chaos-master/releases/download/v0.0.2/chaos-master-0.0.2.linux-amd64.tar.gz
tar -xzf chaos-master-0.0.2.linux-amd64.tar.gz
```

#### Start the chaos master providing a `config.file` that contains the job definitions. 

```bash
./chaos-master --config.file=path/to/config.yml
```
See examples of the file in the `config/example` folder.

```yml
# Contain the configuration for the port and scheme of the api. 
# The deafault values are port: 8080 and scheme: http
api_options:
  port: 8090
  scheme: http

# Contain the definition of all enabled failures. 
# Each failure injection needs to be defined in a job together with the targets that are in scope
jobs:
    # The unique name of the job. The character ',' is not allowed
  - job_name: "docker failure injection"
    # The type of the failure. Can be [Docker, Service, CPU, Server, Network]
    type: "Docker"
    # The name of the target component. Only applicable to Docker and Service failure types
    component_name: "nginx"
    # The list of targets for which is this failure can be applied
    targets: ['host1:8081', 'host2:8081']
  - job_name: "network injection"
    type: "Network"
    targets: ['host1:8081', 'host3:8081']

# Contains the tls configuration for the communication with the bots. 
# If not specified will default to http
# If specified the traffic to the bots will be https
# You can only provide a peer token if the traffic is https
bots:
  # CA certificate
  ca_cert: "config/test/certs/ca-cert.pem"
  # The pub cert for the connection with the bot
  public_cert: "config/test/certs/server-cert.pem"
  # peer token for authorization with the bot. A public cert needs to also be provided
  peer_token: 30028dd6-a641-4ac3-91d8-1e214ac5e6f6

# Contains the configuration for the healthcheck towards the bots
health_check:
  # If set to active the master with send a healthcheck request to the bots every 1 minute
  active: false
  # If set to active the status of the healthcheck will be reported in application log (stderr)
  report: false
```

## API
See the api specification after starting the master at `<host>/chaos/api/v1/swagger/index.html`

## Chaos in practice
1. Define the scope of your experiments. Failure types are scoped to specific targets and components. 
   - For the example config above   
      the `docker` failure is scoped to the `nginx` containers in the targets `'host1:8081', 'host2:8081'`  
      the `network` failure is scoped to targets `'host1:8081', 'host3:8081'`
2. Start a [chaos bots](https://github.com/SotirisAlfonsos/chaos-bot) in each target specified in your jobs 
   - For the example config above  
      we would have to start 3 bots. one on `host1`, one on `host2` and one on `host3`, all on port `8081` 
3. [Optional] Ensure that you have monitoring and alerting in place. Add the recover endpoint as a webhook in case of an alert, to quickly revert all running failures
4. Make the first API call to inject a failure
   - For the example config above  
      ```bash
      curl -ss -X POST "http://127.0.0.1:8090/chaos/api/v1/docker?action=kill" \
      -H "Content-Type: application/json" \
      -d '{"job": "docker failure injection", "containerName": "nginx", "target": "host1:8081"}'
      ```
