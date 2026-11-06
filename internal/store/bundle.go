package store

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dockfin/dockfin/internal/bundle"
	"github.com/dockfin/dockfin/internal/sshx"
	"github.com/google/uuid"
	"golang.org/x/crypto/ssh"
)

const secretBundleVersion = 1

// SecretBundle is the plaintext moved between instances. It is only stored encrypted.
type SecretBundle struct {
	Version    int             `json:"version"`
	Server     SecretServer    `json:"server"`
	PrivateKey SecretKey       `json:"private_key"`
	Projects   []SecretProject `json:"projects"`
	Shared     []SecretShared  `json:"shared"`
}

type SecretServer struct {
	Name         string       `json:"name"`
	Description  string       `json:"description"`
	IP           string       `json:"ip"`
	Port         int          `json:"port"`
	UserName     string       `json:"user_name"`
	ProxyType    string       `json:"proxy_type"`
	Destinations []SecretDest `json:"destinations"`
}

type SecretDest struct {
	Name    string `json:"name"`
	Kind    string `json:"kind"`
	Network string `json:"network"`
}

type SecretKey struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	PrivatePEM  string `json:"private_pem"`
}

type SecretProject struct {
	Name         string      `json:"name"`
	Description  string      `json:"description"`
	Environments []SecretEnv `json:"environments"`
}

type SecretEnv struct {
	Name         string          `json:"name"`
	Description  string          `json:"description"`
	Applications []SecretApp     `json:"applications"`
	Databases    []SecretDB      `json:"databases"`
	Services     []SecretService `json:"services"`
}

type SecretApp struct {
	Name                    string      `json:"name"`
	Description             string      `json:"description"`
	FQDN                    string      `json:"fqdn"`
	BuildPack               string      `json:"build_pack"`
	GitRepository           string      `json:"git_repository"`
	GitBranch               string      `json:"git_branch"`
	DockerfileLocation      string      `json:"dockerfile_location"`
	Dockerfile              string      `json:"dockerfile"`
	DockerComposeLocation   string      `json:"docker_compose_location"`
	DockerComposeRaw        string      `json:"docker_compose_raw"`
	DockerRegistryImageName string      `json:"docker_registry_image_name"`
	DockerRegistryImageTag  string      `json:"docker_registry_image_tag"`
	PortsExposes            string      `json:"ports_exposes"`
	BaseDirectory           string      `json:"base_directory"`
	ComposePrepare          bool        `json:"compose_prepare"`
	DestinationNetwork      string      `json:"destination_network"`
	HTTPBasicAuthUsername   string      `json:"http_basic_auth_username,omitempty"`
	HTTPBasicAuthPassword   string      `json:"http_basic_auth_password,omitempty"`
	WebhookSecret           string      `json:"webhook_secret,omitempty"`
	Env                     []SecretVar `json:"env"`
}

type SecretDB struct {
	Name               string          `json:"name"`
	Description        string          `json:"description"`
	Engine             string          `json:"engine"`
	Image              string          `json:"image"`
	IsPublic           bool            `json:"is_public"`
	PublicPort         *int            `json:"public_port,omitempty"`
	EngineConfig       json.RawMessage `json:"engine_config"`
	Credentials        string          `json:"credentials"`
	DestinationNetwork string          `json:"destination_network"`
	Env                []SecretVar     `json:"env"`
}

type SecretService struct {
	Name               string      `json:"name"`
	Description        string      `json:"description"`
	ServiceType        string      `json:"service_type"`
	DockerComposeRaw   string      `json:"docker_compose_raw"`
	FQDN               string      `json:"fqdn"`
	DestinationNetwork string      `json:"destination_network"`
	Env                []SecretVar `json:"env"`
}

type SecretVar struct {
	Key         string `json:"key"`
	Value       string `json:"value"`
	IsRuntime   bool   `json:"is_runtime"`
	IsBuildtime bool   `json:"is_buildtime"`
	IsLiteral   bool   `json:"is_literal"`
	IsLocked    bool   `json:"is_locked"`
}

type SecretShared struct {
	Scope string `json:"scope"`
	Key   string `json:"key"`
	Value string `json:"value"`
}

type ImportResult struct {
	ServerID  uuid.UUID `json:"server_id"`
	Projects  int       `json:"projects"`
	Apps      int       `json:"applications"`
	Services  int       `json:"services"`
	Databases int       `json:"databases"`
}

func (s *Store) ExportSecretBundle(ctx context.Context, teamID, serverID uuid.UUID, passphrase string) (string, error) {
	doc, err := s.secretDocument(ctx, teamID, serverID)
	if err != nil {
		return "", err
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		return "", err
	}
	return bundle.Seal(passphrase, raw)
}

func (s *Store) secretDocument(ctx context.Context, teamID, serverID uuid.UUID) (*SecretBundle, error) {
	srv, err := s.GetServer(ctx, teamID, serverID)
	if err != nil {
		return nil, err
	}
	doc := &SecretBundle{Version: secretBundleVersion, Projects: []SecretProject{}, Shared: []SecretShared{}}
	doc.Server = SecretServer{
		Name: srv.Name, Description: srv.Description, IP: srv.IP, Port: srv.Port, UserName: srv.UserName, ProxyType: srv.ProxyType,
		Destinations: []SecretDest{},
	}
	if srv.PrivateKeyID != nil {
		key, err := s.GetPrivateKey(ctx, teamID, *srv.PrivateKeyID)
		if err != nil {
			return nil, err
		}
		enc, err := s.GetPrivateKeyMaterial(ctx, teamID, *srv.PrivateKeyID)
		if err != nil {
			return nil, err
		}
		pem, err := s.Box.DecryptString(enc)
		if err != nil {
			return nil, err
		}
		doc.PrivateKey = SecretKey{Name: key.Name, Description: key.Description, PrivatePEM: pem}
	}
	dests, err := s.ListDestinations(ctx, teamID, &serverID)
	if err != nil {
		return nil, err
	}
	netByDest := map[uuid.UUID]string{}
	for _, d := range dests {
		doc.Server.Destinations = append(doc.Server.Destinations, SecretDest{Name: d.Name, Kind: d.Kind, Network: d.Network})
		netByDest[d.ID] = d.Network
	}
	type envKey struct{ project, env uuid.UUID }
	groups := map[envKey]*SecretEnv{}
	order := []envKey{}
	ensure := func(envID uuid.UUID) (*SecretEnv, error) {
		var projectID uuid.UUID
		var pname, pdesc, ename, edesc string
		err := s.Pool.QueryRow(ctx, `
			SELECT p.id, p.name, p.description, e.name, e.description
			FROM environments e JOIN projects p ON p.id=e.project_id
			WHERE e.id=$1 AND e.team_id=$2
		`, envID, teamID).Scan(&projectID, &pname, &pdesc, &ename, &edesc)
		if err != nil {
			return nil, err
		}
		k := envKey{projectID, envID}
		if g, ok := groups[k]; ok {
			return g, nil
		}
		g := &SecretEnv{Name: ename, Description: edesc, Applications: []SecretApp{}, Databases: []SecretDB{}, Services: []SecretService{}}
		groups[k] = g
		order = append(order, k)
		found := false
		for i := range doc.Projects {
			if doc.Projects[i].Name == pname {
				found = true
				break
			}
		}
		if !found {
			doc.Projects = append(doc.Projects, SecretProject{Name: pname, Description: pdesc, Environments: []SecretEnv{}})
		}
		_ = pdesc
		return g, nil
	}
	apps, err := s.ListApplications(ctx, teamID, nil)
	if err != nil {
		return nil, err
	}
	for _, app := range apps {
		if app.DestinationID == nil {
			continue
		}
		if _, ok := netByDest[*app.DestinationID]; !ok {
			continue
		}
		g, err := ensure(app.EnvironmentID)
		if err != nil {
			return nil, err
		}
		item := SecretApp{
			Name: app.Name, Description: app.Description, FQDN: app.FQDN, BuildPack: app.BuildPack,
			GitRepository: app.GitRepository, GitBranch: app.GitBranch,
			DockerfileLocation: app.DockerfileLocation, Dockerfile: app.Dockerfile,
			DockerComposeLocation: app.DockerComposeLocation, DockerComposeRaw: app.DockerComposeRaw,
			DockerRegistryImageName: app.DockerRegistryImageName, DockerRegistryImageTag: app.DockerRegistryImageTag,
			PortsExposes: app.PortsExposes, BaseDirectory: app.BaseDirectory, ComposePrepare: app.ComposePrepare,
			DestinationNetwork: netByDest[*app.DestinationID], HTTPBasicAuthUsername: app.HTTPBasicAuthUsername,
		}
		if app.HTTPBasicAuthPasswordEnc != "" {
			if plain, err := s.Box.DecryptString(app.HTTPBasicAuthPasswordEnc); err == nil {
				item.HTTPBasicAuthPassword = plain
			}
		}
		var hook string
		_ = s.Pool.QueryRow(ctx, `SELECT webhook_secret_enc FROM applications WHERE id=$1`, app.ID).Scan(&hook)
		if hook != "" {
			if plain, err := s.Box.DecryptString(hook); err == nil {
				item.WebhookSecret = plain
			}
		}
		vars, err := s.ListEnvVars(ctx, teamID, "application", app.ID, true)
		if err != nil {
			return nil, err
		}
		item.Env = secretVars(vars)
		g.Applications = append(g.Applications, item)
	}
	dbs, err := s.ListDatabases(ctx, teamID, nil)
	if err != nil {
		return nil, err
	}
	for _, db := range dbs {
		if db.DestinationID == nil {
			continue
		}
		if _, ok := netByDest[*db.DestinationID]; !ok {
			continue
		}
		g, err := ensure(db.EnvironmentID)
		if err != nil {
			return nil, err
		}
		enc, err := s.GetDatabaseCredentials(ctx, teamID, db.ID)
		if err != nil {
			return nil, err
		}
		cred, err := s.Box.DecryptString(enc)
		if err != nil {
			return nil, err
		}
		vars, err := s.ListEnvVars(ctx, teamID, "database", db.ID, true)
		if err != nil {
			return nil, err
		}
		g.Databases = append(g.Databases, SecretDB{
			Name: db.Name, Description: db.Description, Engine: db.Engine, Image: db.Image,
			IsPublic: db.IsPublic, PublicPort: db.PublicPort, EngineConfig: db.EngineConfig,
			Credentials: cred, DestinationNetwork: netByDest[*db.DestinationID], Env: secretVars(vars),
		})
	}
	svcs, err := s.ListServices(ctx, teamID, nil)
	if err != nil {
		return nil, err
	}
	for _, svc := range svcs {
		onServer := svc.ServerID != nil && *svc.ServerID == serverID
		if !onServer && svc.DestinationID != nil {
			_, onServer = netByDest[*svc.DestinationID]
		}
		if !onServer {
			continue
		}
		g, err := ensure(svc.EnvironmentID)
		if err != nil {
			return nil, err
		}
		net := ""
		if svc.DestinationID != nil {
			net = netByDest[*svc.DestinationID]
		}
		vars, err := s.ListEnvVars(ctx, teamID, "service", svc.ID, true)
		if err != nil {
			return nil, err
		}
		g.Services = append(g.Services, SecretService{
			Name: svc.Name, Description: svc.Description, ServiceType: svc.ServiceType,
			DockerComposeRaw: svc.DockerComposeRaw, FQDN: svc.FQDN, DestinationNetwork: net, Env: secretVars(vars),
		})
	}
	projIndex := map[string]int{}
	for i := range doc.Projects {
		projIndex[doc.Projects[i].Name] = i
	}
	for _, k := range order {
		g := groups[k]
		var pname string
		_ = s.Pool.QueryRow(ctx, `SELECT name FROM projects WHERE id=$1`, k.project).Scan(&pname)
		if i, ok := projIndex[pname]; ok {
			doc.Projects[i].Environments = append(doc.Projects[i].Environments, *g)
		}
	}
	shared, err := s.ListSharedEnv(ctx, teamID, "team", nil, true)
	if err != nil {
		return nil, err
	}
	for _, v := range shared {
		doc.Shared = append(doc.Shared, SecretShared{Scope: "team", Key: v.Key, Value: v.Value})
	}
	serverShared, err := s.ListSharedEnv(ctx, teamID, "server", &serverID, true)
	if err == nil {
		for _, v := range serverShared {
			doc.Shared = append(doc.Shared, SecretShared{Scope: "server", Key: v.Key, Value: v.Value})
		}
	}
	return doc, nil
}

func secretVars(in []EnvVar) []SecretVar {
	out := make([]SecretVar, 0, len(in))
	for _, v := range in {
		out = append(out, SecretVar{
			Key: v.Key, Value: v.Value, IsRuntime: v.IsRuntime, IsBuildtime: v.IsBuildtime, IsLiteral: v.IsLiteral, IsLocked: v.IsLocked,
		})
	}
	return out
}

func (s *Store) ImportSecretBundle(ctx context.Context, teamID uuid.UUID, passphrase, envelope string) (*ImportResult, error) {
	plain, err := bundle.Open(passphrase, envelope)
	if err != nil {
		return nil, err
	}
	var doc SecretBundle
	if err := json.Unmarshal(plain, &doc); err != nil {
		return nil, fmt.Errorf("bundle payload is invalid")
	}
	if doc.Version != secretBundleVersion || strings.TrimSpace(doc.Server.Name) == "" || strings.TrimSpace(doc.Server.IP) == "" {
		return nil, fmt.Errorf("bundle payload is incomplete")
	}
	if strings.TrimSpace(doc.PrivateKey.PrivatePEM) == "" {
		return nil, fmt.Errorf("bundle has no private key")
	}
	signer, err := ssh.ParsePrivateKey([]byte(doc.PrivateKey.PrivatePEM))
	if err != nil {
		return nil, fmt.Errorf("bundle private key is invalid")
	}
	pub := string(ssh.MarshalAuthorizedKey(signer.PublicKey()))
	fp := sshx.FingerprintSHA256(signer.PublicKey())
	enc, err := s.Box.EncryptString(doc.PrivateKey.PrivatePEM)
	if err != nil {
		return nil, err
	}
	name := doc.PrivateKey.Name
	if name == "" {
		name = doc.Server.Name + " key"
	}
	key, err := s.CreatePrivateKey(ctx, teamID, name+" (imported)", doc.PrivateKey.Description, pub, enc, fp)
	if err != nil {
		return nil, err
	}
	port := doc.Server.Port
	if port == 0 {
		port = 22
	}
	user := doc.Server.UserName
	if user == "" {
		user = "root"
	}
	proxyType := doc.Server.ProxyType
	if proxyType == "" {
		proxyType = "traefik"
	}
	srv, err := s.CreateServer(ctx, teamID, &key.ID, doc.Server.Name, doc.Server.Description, doc.Server.IP, user, port, proxyType)
	if err != nil {
		return nil, err
	}
	netID := map[string]uuid.UUID{}
	for _, d := range doc.Server.Destinations {
		kind := d.Kind
		if kind == "" {
			kind = "standalone"
		}
		network := d.Network
		if network == "" || network == "dockfin" {
			continue
		}
		var id uuid.UUID
		err := s.Pool.QueryRow(ctx, `
			INSERT INTO destinations (team_id, server_id, name, kind, network)
			VALUES ($1,$2,$3,$4,$5)
			ON CONFLICT (server_id, network) DO UPDATE SET name=EXCLUDED.name
			RETURNING id
		`, teamID, srv.ID, d.Name, kind, network).Scan(&id)
		if err != nil {
			return nil, err
		}
		netID[network] = id
	}
	res := &ImportResult{ServerID: srv.ID}
	for _, p := range doc.Projects {
		pname := p.Name
		proj, _, err := s.CreateProject(ctx, teamID, pname, p.Description)
		if err != nil {
			proj, _, err = s.CreateProject(ctx, teamID, pname+" (imported)", p.Description)
			if err != nil {
				return nil, err
			}
		}
		res.Projects++
		for _, env := range p.Environments {
			ename := env.Name
			if ename == "" {
				ename = "production"
			}
			envs, err := s.ListEnvironments(ctx, teamID, proj.ID)
			if err != nil {
				return nil, err
			}
			var target *Environment
			for i := range envs {
				if envs[i].Name == ename {
					target = &envs[i]
					break
				}
			}
			if target == nil {
				target, err = s.CreateEnvironment(ctx, teamID, proj.ID, ename, env.Description)
				if err != nil {
					return nil, err
				}
			}
			for _, app := range env.Applications {
				var dest *uuid.UUID
				if id, ok := netID[app.DestinationNetwork]; ok {
					dest = &id
				}
				createdApp, err := s.CreateApplication(ctx, &Application{
					TeamID: teamID, EnvironmentID: target.ID, DestinationID: dest,
					Name: app.Name, Description: app.Description, FQDN: app.FQDN, BuildPack: app.BuildPack,
					GitRepository: app.GitRepository, GitBranch: app.GitBranch,
					DockerfileLocation: app.DockerfileLocation, Dockerfile: app.Dockerfile,
					DockerComposeLocation:   app.DockerComposeLocation,
					DockerRegistryImageName: app.DockerRegistryImageName, DockerRegistryImageTag: app.DockerRegistryImageTag,
					PortsExposes: app.PortsExposes, BaseDirectory: app.BaseDirectory, ComposePrepare: app.ComposePrepare,
					HTTPBasicAuthUsername: app.HTTPBasicAuthUsername,
				})
				if err != nil {
					return nil, err
				}
				if app.HTTPBasicAuthPassword != "" {
					penc, err := s.Box.EncryptString(app.HTTPBasicAuthPassword)
					if err != nil {
						return nil, err
					}
					_, _ = s.Pool.Exec(ctx, `UPDATE applications SET http_basic_auth_password_enc=$2 WHERE id=$1`, createdApp.ID, penc)
				}
				if app.WebhookSecret != "" {
					wenc, err := s.Box.EncryptString(app.WebhookSecret)
					if err != nil {
						return nil, err
					}
					_, _ = s.Pool.Exec(ctx, `UPDATE applications SET webhook_secret_enc=$2 WHERE id=$1`, createdApp.ID, wenc)
				}
				if app.DockerComposeRaw != "" {
					_, _ = s.Pool.Exec(ctx, `UPDATE applications SET docker_compose_raw=$2 WHERE id=$1`, createdApp.ID, app.DockerComposeRaw)
				}
				if err := s.importVars(ctx, teamID, "application", createdApp.ID, app.Env); err != nil {
					return nil, err
				}
				res.Apps++
			}
			for _, db := range env.Databases {
				var dest *uuid.UUID
				if id, ok := netID[db.DestinationNetwork]; ok {
					dest = &id
				}
				cenc, err := s.Box.EncryptString(db.Credentials)
				if err != nil {
					return nil, err
				}
				createdDB, err := s.CreateDatabase(ctx, &Database{
					TeamID: teamID, EnvironmentID: target.ID, DestinationID: dest,
					Name: db.Name, Description: db.Description, Engine: db.Engine, Image: db.Image,
					IsPublic: db.IsPublic, PublicPort: db.PublicPort, EngineConfig: db.EngineConfig,
				}, cenc)
				if err != nil {
					return nil, err
				}
				if err := s.importVars(ctx, teamID, "database", createdDB.ID, db.Env); err != nil {
					return nil, err
				}
				res.Databases++
			}
			for _, svc := range env.Services {
				var dest *uuid.UUID
				if id, ok := netID[svc.DestinationNetwork]; ok {
					dest = &id
				}
				sid := srv.ID
				createdSvc, err := s.CreateService(ctx, &Service{
					TeamID: teamID, EnvironmentID: target.ID, ServerID: &sid, DestinationID: dest,
					Name: svc.Name, Description: svc.Description, ServiceType: svc.ServiceType,
					DockerComposeRaw: svc.DockerComposeRaw, FQDN: svc.FQDN,
				})
				if err != nil {
					return nil, err
				}
				if err := s.importVars(ctx, teamID, "service", createdSvc.ID, svc.Env); err != nil {
					return nil, err
				}
				res.Services++
			}
		}
	}
	for _, sh := range doc.Shared {
		scope := sh.Scope
		var scopeID *uuid.UUID
		if scope == "server" {
			scopeID = &srv.ID
		} else {
			scope = "team"
		}
		if _, err := s.UpsertSharedEnv(ctx, teamID, scope, scopeID, sh.Key, sh.Value, false); err != nil {
			return nil, err
		}
	}
	return res, nil
}

func (s *Store) importVars(ctx context.Context, teamID uuid.UUID, kind string, id uuid.UUID, vars []SecretVar) error {
	for _, v := range vars {
		if strings.TrimSpace(v.Key) == "" {
			continue
		}
		if _, err := s.UpsertEnvVar(ctx, teamID, kind, id, UpsertEnvVarInput{
			Key: v.Key, Value: v.Value, Runtime: v.IsRuntime, Buildtime: v.IsBuildtime, Literal: v.IsLiteral, Locked: v.IsLocked,
		}); err != nil {
			return err
		}
	}
	return nil
}
