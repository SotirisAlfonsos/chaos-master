package network

import (
	"context"

	v1 "github.com/SotirisAlfonsos/chaos-bot/proto/grpc/v1"
	"google.golang.org/grpc"
)

type MockConnection struct {
	Status *v1.StatusResponse
	Err    error
}

func (connection *MockConnection) GetServiceClient() (v1.ServiceClient, error) {
	return GetMockServiceClient(connection.Status, connection.Err), nil
}

func (connection *MockConnection) GetDockerClient() (v1.DockerClient, error) {
	return GetMockDockerClient(connection.Status, connection.Err), nil
}

func (connection *MockConnection) GetCPUClient() (v1.CPUClient, error) {
	return GetMockCPUClient(connection.Status, connection.Err), nil
}

func (connection *MockConnection) GetServerClient() (v1.ServerClient, error) {
	return GetMockServerClient(connection.Status, connection.Err), nil
}

func (connection *MockConnection) GetNetworkClient() (v1.NetworkClient, error) {
	return GetMockNetworkClient(connection.Status, connection.Err), nil
}

func (connection *MockConnection) GetHealthClient() (v1.HealthClient, error) {
	return nil, nil
}

type MockFailedConnection struct {
	Err error
}

func (failedConnection *MockFailedConnection) GetServiceClient() (v1.ServiceClient, error) {
	return nil, failedConnection.Err
}

func (failedConnection *MockFailedConnection) GetDockerClient() (v1.DockerClient, error) {
	return nil, failedConnection.Err
}

func (failedConnection *MockFailedConnection) GetCPUClient() (v1.CPUClient, error) {
	return nil, failedConnection.Err
}

func (failedConnection *MockFailedConnection) GetServerClient() (v1.ServerClient, error) {
	return nil, failedConnection.Err
}

func (failedConnection *MockFailedConnection) GetNetworkClient() (v1.NetworkClient, error) {
	return nil, failedConnection.Err
}
func (failedConnection *MockFailedConnection) GetHealthClient() (v1.HealthClient, error) {
	return nil, failedConnection.Err
}

type mockServiceClient struct {
	Status *v1.StatusResponse
	Error  error
}

func GetMockServiceClient(status *v1.StatusResponse, err error) v1.ServiceClient {
	return &mockServiceClient{Status: status, Error: err}
}

func (msc *mockServiceClient) Recover(_ context.Context, _ *v1.ServiceRequest, _ ...grpc.CallOption) (*v1.StatusResponse, error) {
	return msc.Status, msc.Error
}

func (msc *mockServiceClient) Kill(_ context.Context, _ *v1.ServiceRequest, _ ...grpc.CallOption) (*v1.StatusResponse, error) {
	return msc.Status, msc.Error
}

type mockDockerClient struct {
	Status *v1.StatusResponse
	Error  error
}

func GetMockDockerClient(status *v1.StatusResponse, err error) v1.DockerClient {
	return &mockDockerClient{Status: status, Error: err}
}

func (msc *mockDockerClient) Recover(_ context.Context, _ *v1.DockerRequest, _ ...grpc.CallOption) (*v1.StatusResponse, error) {
	return msc.Status, msc.Error
}

func (msc *mockDockerClient) Kill(_ context.Context, _ *v1.DockerRequest, _ ...grpc.CallOption) (*v1.StatusResponse, error) {
	return msc.Status, msc.Error
}

type mockCPUClient struct {
	Status *v1.StatusResponse
	Error  error
}

func GetMockCPUClient(status *v1.StatusResponse, err error) v1.CPUClient {
	return &mockCPUClient{Status: status, Error: err}
}

func (mcc *mockCPUClient) Start(_ context.Context, _ *v1.CPURequest, _ ...grpc.CallOption) (*v1.StatusResponse, error) {
	return mcc.Status, mcc.Error
}

func (mcc *mockCPUClient) Recover(_ context.Context, _ *v1.CPURequest, _ ...grpc.CallOption) (*v1.StatusResponse, error) {
	return mcc.Status, mcc.Error
}

type mockServerClient struct {
	Status *v1.StatusResponse
	Error  error
}

func GetMockServerClient(status *v1.StatusResponse, err error) v1.ServerClient {
	return &mockServerClient{Status: status, Error: err}
}

func (msc *mockServerClient) Kill(_ context.Context, _ *v1.ServerRequest, _ ...grpc.CallOption) (*v1.StatusResponse, error) {
	return msc.Status, msc.Error
}

type mockNetworkClient struct {
	Status *v1.StatusResponse
	Error  error
}

func GetMockNetworkClient(status *v1.StatusResponse, err error) v1.NetworkClient {
	return &mockNetworkClient{Status: status, Error: err}
}

func (mcc *mockNetworkClient) Start(_ context.Context, _ *v1.NetworkRequest, _ ...grpc.CallOption) (*v1.StatusResponse, error) {
	return mcc.Status, mcc.Error
}

func (mcc *mockNetworkClient) Recover(_ context.Context, _ *v1.NetworkRequest, _ ...grpc.CallOption) (*v1.StatusResponse, error) {
	return mcc.Status, mcc.Error
}
