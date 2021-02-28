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

func (connection *MockConnection) GetServiceClient(string) (v1.ServiceClient, error) {
	return GetMockServiceClient(connection.Status, connection.Err), nil
}

func (connection *MockConnection) GetDockerClient(string) (v1.DockerClient, error) {
	return GetMockDockerClient(connection.Status, connection.Err), nil
}

func (connection *MockConnection) GetCPUClient(string) (v1.CPUClient, error) {
	return GetMockCPUClient(connection.Status, connection.Err), nil
}

func (connection *MockConnection) GetServerClient(string) (v1.ServerClient, error) {
	return GetMockServerClient(connection.Status, connection.Err), nil
}

func (connection *MockConnection) GetHealthClient(string) (v1.HealthClient, error) {
	return nil, nil
}

type MockFailedConnection struct {
	Err error
}

func (failedConnection *MockFailedConnection) GetServiceClient(string) (v1.ServiceClient, error) {
	return nil, failedConnection.Err
}

func (failedConnection *MockFailedConnection) GetDockerClient(string) (v1.DockerClient, error) {
	return nil, failedConnection.Err
}

func (failedConnection *MockFailedConnection) GetCPUClient(string) (v1.CPUClient, error) {
	return nil, failedConnection.Err
}

func (failedConnection *MockFailedConnection) GetServerClient(string) (v1.ServerClient, error) {
	return nil, failedConnection.Err
}

func (failedConnection *MockFailedConnection) GetHealthClient(string) (v1.HealthClient, error) {
	return nil, failedConnection.Err
}

type mockServiceClient struct {
	Status *v1.StatusResponse
	Error  error
}

func GetMockServiceClient(status *v1.StatusResponse, err error) v1.ServiceClient {
	return &mockServiceClient{Status: status, Error: err}
}

func (msc *mockServiceClient) Start(_ context.Context, _ *v1.ServiceRequest, _ ...grpc.CallOption) (*v1.StatusResponse, error) {
	return msc.Status, msc.Error
}

func (msc *mockServiceClient) Stop(_ context.Context, _ *v1.ServiceRequest, _ ...grpc.CallOption) (*v1.StatusResponse, error) {
	return msc.Status, msc.Error
}

type mockDockerClient struct {
	Status *v1.StatusResponse
	Error  error
}

func GetMockDockerClient(status *v1.StatusResponse, err error) v1.DockerClient {
	return &mockDockerClient{Status: status, Error: err}
}

func (msc *mockDockerClient) Start(_ context.Context, _ *v1.DockerRequest, _ ...grpc.CallOption) (*v1.StatusResponse, error) {
	return msc.Status, msc.Error
}

func (msc *mockDockerClient) Stop(_ context.Context, _ *v1.DockerRequest, _ ...grpc.CallOption) (*v1.StatusResponse, error) {
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

func (mcc *mockCPUClient) Stop(_ context.Context, _ *v1.CPURequest, _ ...grpc.CallOption) (*v1.StatusResponse, error) {
	return mcc.Status, mcc.Error
}

type mockServerClient struct {
	Status *v1.StatusResponse
	Error  error
}

func GetMockServerClient(status *v1.StatusResponse, err error) v1.ServerClient {
	return &mockServerClient{Status: status, Error: err}
}

func (msc *mockServerClient) Stop(_ context.Context, _ *v1.ServerRequest, _ ...grpc.CallOption) (*v1.StatusResponse, error) {
	return msc.Status, msc.Error
}
