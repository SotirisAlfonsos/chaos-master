package network

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"

	"github.com/SotirisAlfonsos/chaos-master/pkg/chaoslogger"

	v1 "github.com/SotirisAlfonsos/chaos-bot/proto/grpc/v1"
	"github.com/SotirisAlfonsos/chaos-master/config"
	"github.com/go-kit/kit/log/level"
	"github.com/pkg/errors"
	"golang.org/x/oauth2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/oauth"
)

type Connections struct {
	Pool map[string]Connection
}

type Connection interface {
	GetServiceClient() (v1.ServiceClient, error)
	GetDockerClient() (v1.DockerClient, error)
	GetCPUClient() (v1.CPUClient, error)
	GetServerClient() (v1.ServerClient, error)
	GetNetworkClient() (v1.NetworkClient, error)
	GetHealthClient() (v1.HealthClient, error)
}

type connection struct {
	target           string
	clientConnection *grpc.ClientConn
	options          *Options
	loggers          chaoslogger.Loggers
}

type Options struct {
	cACert     string
	publicCert string
	peerToken  string
}

func GetConnectionPool(config *config.Config, loggers chaoslogger.Loggers) *Connections {
	connections := &Connections{
		Pool: make(map[string]Connection),
	}

	options := &Options{}

	if config.Bots != nil {
		options.peerToken = config.Bots.PeerToken
		options.cACert = config.Bots.CACert
		options.publicCert = config.Bots.PublicCert
	}

	for _, jobFromConfig := range config.JobsFromConfig {
		connections.addForTargets(jobFromConfig.Targets, options, loggers)
	}

	return connections
}

func (connections *Connections) addForTargets(targets []string, options *Options, loggers chaoslogger.Loggers) {
	for _, target := range targets {
		connection := &connection{target: target, options: options, loggers: loggers}
		if err := connection.addToPool(connections, target); err != nil {
			_ = level.Error(loggers.ErrLogger).Log("msg", fmt.Sprintf("failed to add connection to target %s, to connection pool", target), "err", err)
		}
	}
}

func (connection *connection) addToPool(connections *Connections, target string) error {
	if _, ok := connections.Pool[target]; !ok {
		connections.Pool[target] = connection
		err := connection.dial()
		if err != nil {
			return err
		}
	}
	return nil
}

func (connection *connection) dial() error {
	if connection.clientConnection == nil ||
		(connection.clientConnection.GetState() != connectivity.Ready &&
			connection.clientConnection.GetState() != connectivity.Connecting) {
		err := connection.updateClientConnection()
		if err != nil {
			return errors.Wrap(err, "could not establish client connection")
		}
	}
	return nil
}

func (connection *connection) updateClientConnection() error {
	opts, err := connection.options.getGRPCOptions()
	if err != nil {
		return err
	}

	_ = level.Info(connection.loggers.OutLogger).Log("msg", fmt.Sprintf("Dial %s ...", connection.target))
	clientConn, err := grpc.Dial(connection.target, opts...)
	if err != nil {
		return err
	}

	connection.clientConnection = clientConn

	return nil
}

func (options *Options) getGRPCOptions() ([]grpc.DialOption, error) {
	opts := make([]grpc.DialOption, 0)

	if options.peerToken == "" && options.cACert == "" && options.publicCert == "" {
		return append(opts, grpc.WithInsecure()), nil
	}

	if options.peerToken != "" {
		rpcToken := oauth.NewOauthAccess(&oauth2.Token{AccessToken: options.peerToken})
		opts = append(opts, grpc.WithPerRPCCredentials(rpcToken))
	}

	if options.cACert != "" {
		tlsCredentials, err := loadTLSCredentials(options.cACert)
		if err != nil {
			return nil, fmt.Errorf("cannot load ca cert: %s", err)
		}
		opts = append(opts, grpc.WithTransportCredentials(tlsCredentials))
	} else if options.publicCert != "" {
		tlsCredentials, err := loadTLSCredentials(options.publicCert)
		if err != nil {
			return nil, fmt.Errorf("could not load public cert: %s", err)
		}
		opts = append(opts, grpc.WithTransportCredentials(tlsCredentials))
	}

	return opts, nil
}

func loadTLSCredentials(cert string) (credentials.TransportCredentials, error) {
	// Load certificate of the CA who signed server's certificate
	pemCert, err := ioutil.ReadFile(cert)
	if err != nil {
		return nil, err
	}

	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(pemCert) {
		return nil, fmt.Errorf("failed to add pem certificate to cert pool")
	}

	// Create the credentials and return it
	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
		RootCAs:    certPool,
	}

	return credentials.NewTLS(tlsConfig), nil
}

func (connection *connection) GetServiceClient() (v1.ServiceClient, error) {
	err := connection.dial()
	if err != nil {
		return nil, err
	}
	return v1.NewServiceClient(connection.clientConnection), nil
}

func (connection *connection) GetDockerClient() (v1.DockerClient, error) {
	err := connection.dial()
	if err != nil {
		return nil, err
	}
	return v1.NewDockerClient(connection.clientConnection), nil
}

func (connection *connection) GetCPUClient() (v1.CPUClient, error) {
	err := connection.dial()
	if err != nil {
		return nil, err
	}
	return v1.NewCPUClient(connection.clientConnection), nil
}

func (connection *connection) GetServerClient() (v1.ServerClient, error) {
	err := connection.dial()
	if err != nil {
		return nil, err
	}
	return v1.NewServerClient(connection.clientConnection), nil
}

func (connection *connection) GetNetworkClient() (v1.NetworkClient, error) {
	err := connection.dial()
	if err != nil {
		return nil, err
	}
	return v1.NewNetworkClient(connection.clientConnection), nil
}

func (connection *connection) GetHealthClient() (v1.HealthClient, error) {
	err := connection.dial()
	if err != nil {
		return nil, err
	}
	return v1.NewHealthClient(connection.clientConnection), nil
}
