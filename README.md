# Chaos Master

[![Build Status](https://travis-ci.org/SotirisAlfonsos/chaos-master.svg)](https://travis-ci.org/SotirisAlfonsos/chaos-master)
[![Go Report Card](https://goreportcard.com/badge/github.com/SotirisAlfonsos/chaos-master)](https://goreportcard.com/report/github.com/SotirisAlfonsos/chaos-master)
[![codebeat badge](https://codebeat.co/badges/ab1778ae-60c1-4b7d-aff6-a8f1eabbd2d5)](https://codebeat.co/projects/github-com-sotirisalfonsos-chaos-master-master)
[![codecov.io](https://codecov.io/github/SotirisAlfonsos/chaos-master/coverage.svg?branch=master)](https://codecov.io/github/SotirisAlfonsos/chaos-master?branch=master)

The master provides an api to send fault injections to the [chaos bots](https://github.com/SotirisAlfonsos/chaos-bot)
#### [Chaos in practice](https://principlesofchaos.org/)
1. Start by defining a ‘steady state’.
2. Hypothesize that this steady state will continue in both the control group and the experimental group.
3. <b>Inject failures that reflect real world events.</b>
4. <i>Try to disprove the hypothesis by looking for a difference in steady state between the control group and the experimental group.</i>  

At this point we should add one more stage

5. <b>Recover fast to the ‘steady state’.</b>

The master and bots focus on two of the stages of chaos, stages <b>3</b> and <b>5</b>

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
    targets: ['127.0.0.1:8083', '127.0.0.1:8081']
  - job_name: "network injection"
    type: "Network"
    targets: ['127.0.0.1:8081']

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

### API
See the api specification after starting the master at `\<host\>/chaos/api/v1/swagger/index.html`
