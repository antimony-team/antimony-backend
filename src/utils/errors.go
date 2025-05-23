package utils

import "errors"

var ErrorRunningLab = errors.New("modifications to running lab are not allowed")
var ErrorContainerlab = errors.New("containerlab subprocess failed")
var ErrorOpenIDError = errors.New("failed to authenticate via openid connect")
var ErrorOpenIDDisabledError = errors.New("authentication via openid is disabled")
var ErrorFileStorage = errors.New("filesystem read or write error")
var ErrorUuidNotFound = errors.New("the specified uuid was not found")
var ErrorNodeNotFound = errors.New("the specified node was not found")
var ErrorTokenInvalid = errors.New("the auth token provided was invalid")
var ErrorTopologyExists = errors.New("a topology with that name already exists in that collection")
var ErrorBindFileExists = errors.New("a file with that path already exists for that topology")
var ErrorCollectionExists = errors.New("a collection with that name already exists")
var ErrorInvalidCredentials = errors.New("the credentials provided were invalid")

var ErrorUnauthorized = errors.New("the request was unauthorized")
var ErrorForbidden = errors.New("access to the requested action is forbidden")
var ErrorNoWriteAccessToLab = errors.New("write access to the provided lab is not granted")
var ErrorNoWriteAccessToBindFile = errors.New("write access to the provided file is not granted")
var ErrorNoWriteAccessToTopology = errors.New("write access to the provided topology is not granted")
var ErrorNoWriteAccessToCollection = errors.New("write access to the provided collection is not granted")
var ErrorNoDeployAccessToLab = errors.New("deploy access to the provided lab is not granted")
var ErrorNoDestroyAccessToLab = errors.New("destroy access to the provided lab is not granted")
var ErrorNoDeployAccessToCollection = errors.New("deploy access to the provided collection is not granted")
var ErrorNoPermissionToCreateCollections = errors.New("permission to create collections is not granted")

var ErrorLabNotRunning = errors.New("the specified lab is not running")
var ErrorLabActionInProgress = errors.New("there is already an action in progress for the specified lab")
var ErrorLabAlreadyRunning = errors.New("the specified lab is already running")
var ErrorInvalidLabCommand = errors.New("the provided lab command was invalid")
var ErrorInvalidSocketRequest = errors.New("the socket request was invalid")
