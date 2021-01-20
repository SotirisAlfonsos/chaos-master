[![Build Status](https://travis-ci.org/SotirisAlfonsos/chaos-master.svg)](https://travis-ci.org/SotirisAlfonsos/chaos-master)
[![Go Report Card](https://goreportcard.com/badge/github.com/SotirisAlfonsos/chaos-master)](https://goreportcard.com/report/github.com/SotirisAlfonsos/chaos-master)
[![codebeat badge](https://codebeat.co/badges/ab1778ae-60c1-4b7d-aff6-a8f1eabbd2d5)](https://codebeat.co/projects/github-com-sotirisalfonsos-chaos-master-master)
[![codecov.io](https://codecov.io/github/SotirisAlfonsos/chaos-master/coverage.svg?branch=master)](https://codecov.io/github/SotirisAlfonsos/chaos-master?branch=master)

# chaos-master
The master provides an api to send fault injections to the [chaos bots](https://github.com/SotirisAlfonsos/chaos-bot)

#### Chaos in practice
1. Start by defining a ‘steady state’.
2. Hypothesize that this steady state will continue in both the control group and the experimental group.
3. <b>Inject failures that reflect real world events.</b>
4. <i>Try to disprove the hypothesis by looking for a difference in steady state between the control group and the experimental group.</i>
5. <b>Recover fast to the ‘steady state’.</b>

More info on https://principlesofchaos.org/

The master and bots focus on points <b>3</b> and <b>5</b>

## Starting Up
Start the chaos master providing a <i>config.file</i> that contains the job definitions. See an example of the file in the <i>config/example</i> folder.

### API
See the api specification after starting the master at <i>\<host\>/chaos/api/v1/swagger</i>