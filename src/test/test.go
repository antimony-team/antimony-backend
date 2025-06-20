package test

import (
	"antimonyBackend/auth"
	"antimonyBackend/config"
	"antimonyBackend/deployment"
	"antimonyBackend/domain/collection"
	"antimonyBackend/domain/device"
	"antimonyBackend/domain/lab"
	"antimonyBackend/domain/schema"
	"antimonyBackend/domain/statusMessage"
	"antimonyBackend/domain/topology"
	"antimonyBackend/domain/user"
	"antimonyBackend/socket"
	"antimonyBackend/storage"
	"antimonyBackend/utils"
	"context"
	"io"
	"testing"
	"time"

	"github.com/charmbracelet/log"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func ptr(t time.Time) *time.Time { return &t }

//nolint:funlen
func GenerateTestData(db *gorm.DB, storage storage.StorageManager) {
	user1 := user.User{
		UUID: "test-user-id1",
		Sub:  "doesntmatter",
		Name: "Hans Hülsensack",
	}
	if err := db.Create(&user1).Error; err != nil {
		log.Fatalf("Create user1 failed: %v", err)
	}
	user2 := user.User{
		UUID: "test-user-id2",
		Sub:  "doesntmatter",
		Name: "Max Muster",
	}
	db.Create(&user2)
	user3 := user.User{
		UUID: "test-user-id3",
		Sub:  "irrelevant",
		Name: "Lu",
	}
	db.Create(&user3)
	user4 := user.User{
		UUID: "test-user-id4",
		Sub:  "irrelevant",
		Name: "Emergency Döner",
	}
	db.Create(&user4)
	user5 := user.User{
		UUID: "test-user-id5",
		Sub:  "irrelevant",
		Name: "Not Authenticated",
	}
	db.Create(&user5)

	db.Create(&collection.Collection{
		UUID:         utils.GenerateUuid(),
		Name:         "hidden-group",
		PublicWrite:  true,
		PublicDeploy: false,
		Creator:      user1,
	})

	db.Create(&collection.Collection{
		UUID:         utils.GenerateUuid(),
		Name:         "fs25-cldinf",
		PublicWrite:  false,
		PublicDeploy: false,
		Creator:      user1,
	})

	db.Create(&collection.Collection{
		UUID:         utils.GenerateUuid(),
		Name:         "fs25-nisec",
		PublicWrite:  true,
		PublicDeploy: false,
		Creator:      user1,
	})

	db.Create(&collection.Collection{
		UUID:         utils.GenerateUuid(),
		Name:         "hs25-cn1",
		PublicWrite:  false,
		PublicDeploy: true,
		Creator:      user1,
	})

	collection1 := collection.Collection{
		UUID:         "CollectionTestUUID1",
		Name:         "hs25-cn2",
		PublicWrite:  true,
		PublicDeploy: true,
		Creator:      user1,
	}
	db.Create(&collection1)

	topology1Uuid := "TopologyTestUUID1"
	topology1 := topology.Topology{
		UUID:       topology1Uuid,
		Name:       "ctd",
		SyncUrl:    "",
		Collection: collection1,
		Creator:    user1,
	}
	if err := db.Create(&topology1).Error; err != nil {
		log.Fatalf("Create topology failed: %v", err)
	}

	db.Create(&topology.BindFile{
		UUID:       "BindFileTestUUID1",
		FilePath:   "leaf01/interfaces",
		Topology:   topology1,
		TopologyID: topology1.ID,
	})
	db.Create(&topology.BindFile{
		UUID:       utils.GenerateUuid(),
		FilePath:   "leaf01/daemons",
		Topology:   topology1,
		TopologyID: topology1.ID,
	})
	db.Create(&topology.BindFile{
		UUID:       utils.GenerateUuid(),
		FilePath:   "leaf01/frr.conf",
		Topology:   topology1,
		TopologyID: topology1.ID,
	})

	writeTopologyFile(topology1Uuid, cvx03, storage)
	writeBindFile(topology1Uuid, "leaf01/interfaces", "1", storage)
	writeBindFile(topology1Uuid, "leaf01/daemons", "2", storage)
	writeBindFile(topology1Uuid, "leaf01/frr.conf", "3", storage)

	topology2Uuid := utils.GenerateUuid()
	topology2 := topology.Topology{
		UUID:       topology2Uuid,
		Name:       "test1",
		SyncUrl:    "",
		Collection: collection1,
		Creator:    user1,
	}
	db.Create(&topology2)
	writeTopologyFile(topology2Uuid, test1, storage)
	emptyString := "empty"
	lab1 := lab.Lab{
		UUID:               "TestLabUUID1",
		Name:               "Test Lab",
		StartTime:          time.Now().Add(-1 * time.Hour),
		EndTime:            ptr(time.Now().Add(1 * time.Hour)),
		TopologyID:         topology1.ID,
		Topology:           topology1,
		CreatorID:          user1.ID,
		Creator:            user1,
		TopologyDefinition: &emptyString,
	}
	db.Create(&lab1)
	lab2 := lab.Lab{
		UUID:               "TestLabUUID2",
		Name:               "Test Lab2",
		StartTime:          time.Now().Add(-1 * time.Hour),
		EndTime:            ptr(time.Now().Add(1 * time.Hour)),
		TopologyID:         topology1.ID,
		Topology:           topology1,
		CreatorID:          user1.ID,
		Creator:            user1,
		TopologyDefinition: &emptyString,
	}
	db.Create(&lab2)
}

const cvx03 = `name: ctd
topology:
  nodes:
    leaf01:
      kind: nokia_srlinux
      image: networkop/cx:4.3.0
      binds:
        - leaf01/interfaces:/etc/network/interfaces
        - leaf01/daemons:/etc/frr/daemons
        - leaf01/frr.conf:/etc/frr/frr.conf

    leaf02:
      kind: nokia_srlinux
      image: networkop/cx:4.3.0

    spine01:
      kind: nokia_srlinux
      image: networkop/cx:4.3.0

    server01:
      kind: linux
      image: networkop/host:ifreload

    server02:
      kind: linux
      image: networkop/host:ifreload


  links:
    - endpoints: ["leaf01:swp1", "server01:eth1"]
    - endpoints: ["leaf01:swp2", "server02:eth1"]
    - endpoints: ["leaf02:swp1", "server01:eth2"]
    - endpoints: ["leaf02:swp2", "server02:eth2"]

    - endpoints: ["leaf01:swp49", "leaf02:swp49"]
    - endpoints: ["leaf01:swp50", "leaf02:swp50"]

    - endpoints: ["spine01:swp1", "leaf01:swp51"]
    - endpoints: ["spine01:swp2", "leaf02:swp51"]`

const test1 = `name: test1
topology:
  nodes:
    node1:
      kind: nokia_srlinux
      image: ghcr.io/nokia/srlinux

    node2:
      kind: nokia_srlinux
      image: ghcr.io/nokia/srlinux

  links:
    - endpoints: ["node1:e1-1", "node2:e1-1"]`

func writeTopologyFile(topologyId string, content string, storage storage.StorageManager) {
	if err := storage.WriteTopology(topologyId, content); err != nil {
		log.Fatalf("Failed to write test topology: %s", err.Error())
	}
}

func writeBindFile(topologyId string, filePath string, content string, storage storage.StorageManager) {
	if err := storage.WriteBindFile(topologyId, filePath, content); err != nil {
		log.Fatalf("Failed to write test topology bind file: %s", err.Error())
	}
}

func addAuthenticatedUsers(authManager auth.AuthManager) {
	var err error

	_, err = authManager.RegisterTestUser(auth.AuthenticatedUser{
		UserId:      "test-user-id1",
		IsAdmin:     true,
		Collections: []string{"hidden-group", "fs25-cldinf", "fs25-nisec", "hs25-cn1", "hs25-cn2"},
	})
	if err != nil {
		return
	}

	_, err = authManager.RegisterTestUser(auth.AuthenticatedUser{
		UserId:      "test-user-id2",
		IsAdmin:     true,
		Collections: []string{},
	})
	if err != nil {
		return
	}

	_, err = authManager.RegisterTestUser(auth.AuthenticatedUser{
		UserId:      "test-user-id3",
		IsAdmin:     false,
		Collections: []string{"hidden-group", "fs25-cldinf", "fs25-nisec", "hs25-cn1", "hs25-cn2"},
	})
	if err != nil {
		return
	}

	_, err = authManager.RegisterTestUser(auth.AuthenticatedUser{
		UserId:      "test-user-id4",
		IsAdmin:     false,
		Collections: []string{},
	})
	if err != nil {
		return
	}
}

type MockDeploymentProvider struct{}

func (p *MockDeploymentProvider) Deploy(
	ctx context.Context,
	topologyFile string,
	onLog func(data string),
) (*string, error) {
	output := "Mock Deploy called"
	if onLog != nil {
		onLog(output)
	}
	return &output, nil
}

func (p *MockDeploymentProvider) Redeploy(
	ctx context.Context,
	topologyFile string,
	onLog func(data string),
) (*string, error) {
	output := "Mock Redeploy called"
	if onLog != nil {
		onLog(output)
	}
	return &output, nil
}

func (p *MockDeploymentProvider) Destroy(
	ctx context.Context,
	topologyFile string,
	onLog func(data string),
) (*string, error) {
	output := "Mock Destroy called"
	if onLog != nil {
		onLog(output)
	}
	return &output, nil
}

func (p *MockDeploymentProvider) Inspect(
	ctx context.Context,
	topologyFile string,
	onLog func(data string),
) (deployment.InspectOutput, error) {
	// Return empty output and nil error as mock
	return deployment.InspectOutput{}, nil
}

func (p *MockDeploymentProvider) InspectAll(ctx context.Context) (deployment.InspectOutput, error) {
	return deployment.InspectOutput{}, nil
}

func (p *MockDeploymentProvider) Exec(
	ctx context.Context,
	topologyFile string,
	content string,
	onLog func(data string),
	onDone func(output *string, err error),
) {
	if onLog != nil {
		onLog("Mock Exec called")
	}
	if onDone != nil {
		output := "Mock Exec result"
		onDone(&output, nil)
	}
}

func (p *MockDeploymentProvider) ExecOnNode(
	ctx context.Context,
	topologyFile string,
	content string,
	nodeLabel string,
	onLog func(data string),
	onDone func(output *string, err error),
) {
	if onLog != nil {
		onLog("Mock ExecOnNode called")
	}
	if onDone != nil {
		output := "Mock ExecOnNode result"
		onDone(&output, nil)
	}
}

func (p *MockDeploymentProvider) OpenShell(ctx context.Context, containerId string) (io.ReadWriteCloser, error) {
	// Return nil or a dummy ReadWriteCloser if needed.
	//nolint:nilnil // This is a mock
	return nil, nil
}

func (p *MockDeploymentProvider) StartNode(ctx context.Context, containerId string) error {
	return nil
}

func (p *MockDeploymentProvider) StopNode(ctx context.Context, containerId string) error {
	return nil
}

func (p *MockDeploymentProvider) RestartNode(ctx context.Context, containerId string) error {
	return nil
}

func (p *MockDeploymentProvider) RegisterListener(ctx context.Context, onUpdate func(containerId string)) error {
	// Simulate some fake events if needed, otherwise just return nil.
	return nil
}

func (p *MockDeploymentProvider) StreamContainerLogs(
	ctx context.Context,
	topologyFile string,
	containerId string,
	onLog func(data string),
) error {
	if onLog != nil {
		onLog("Mock log line")
	}
	return nil
}

func SetupTestServer(t *testing.T) (*gin.Engine, auth.AuthManager, *gorm.DB) {
	gin.SetMode(gin.TestMode)

	// Set environment variables
	t.Setenv("SB_NATIVE_USERNAME", "testuser")
	t.Setenv("SB_NATIVE_PASSWORD", "testpass")

	storageDir := t.TempDir()
	runDir := t.TempDir()
	// Load config manually
	cfg := &config.AntimonyConfig{
		FileSystem: config.FilesystemConfig{
			Storage: storageDir,
			Run:     runDir,
		},
		Containerlab: config.ClabConfig{
			SchemaUrl:      "",
			SchemaFallback: "../../data/clab.schema.json",
			DeviceConfig:   "../../data/device-config.json",
		},
		Auth: config.AuthConfig{
			EnableNative:      true,
			EnableOpenID:      false,
			OpenIdIssuer:      "",
			OpenIdClientID:    "",
			OpenIdAdminGroups: []string{},
		},
	}
	authManager := auth.CreateAuthManager(cfg)
	devicesService := device.CreateService(cfg)
	storageManager := storage.CreateStorageManager(cfg)
	schemaService := schema.CreateService(cfg)

	// Step 4: Setup in-memory DB
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = db.AutoMigrate(
		&user.User{},
		&collection.Collection{},
		&topology.Topology{},
		&topology.BindFile{},
		&lab.Lab{},
	)
	require.NoError(t, err)

	// Seed test data
	GenerateTestData(db, storageManager)

	// Init repos, services, handlers
	socketManager := socket.CreateSocketManager(authManager)

	statusMessageNamespace := socket.CreateOutputNamespace[statusMessage.StatusMessage](
		socketManager, false, false, false, nil, "status-messages",
	)

	userRepo := user.CreateRepository(db)
	userService := user.CreateService(userRepo, authManager)
	userHandler := user.CreateHandler(userService)

	devicesHandler := device.CreateHandler(devicesService)

	schemaHandler := schema.CreateHandler(schemaService)

	mockProvider := &MockDeploymentProvider{}

	collectionRepo := collection.CreateRepository(db)
	collectionService := collection.CreateService(collectionRepo, userRepo)
	collectionHandler := collection.CreateHandler(collectionService)

	topologyRepository := topology.CreateRepository(db)
	topologyService := topology.CreateService(
		topologyRepository, userRepo, collectionRepo,
		storageManager, schemaService.Get(),
	)
	topologyHandler := topology.CreateHandler(topologyService)

	labRepository := lab.CreateRepository(db)
	labService := lab.CreateService(
		cfg, labRepository, userRepo, topologyRepository, topologyService,
		storageManager, socketManager, statusMessageNamespace, mockProvider,
	)
	labHandler := lab.CreateHandler(labService)

	addAuthenticatedUsers(authManager)

	_, err = authManager.RegisterTestUser(auth.AuthenticatedUser{
		UserId:      auth.NativeUserID,
		IsAdmin:     true,
		Collections: []string{"hidden-group", "fs25-cldinf", "fs25-nisec", "hs25-cn1", "hs25-cn2"},
	})

	if err != nil {
		log.Fatalf("Failed to register test user")
	}

	// Setup Gin + register routes with real middleware
	router := gin.Default()
	collection.RegisterRoutes(router, collectionHandler, authManager)
	device.RegisterRoutes(router, devicesHandler, authManager)
	topology.RegisterRoutes(router, topologyHandler, authManager)
	schema.RegisterRoutes(router, schemaHandler)
	user.RegisterRoutes(router, userHandler)
	lab.RegisterRoutes(router, labHandler, authManager)
	return router, authManager, db
}
