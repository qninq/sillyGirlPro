package core

import (
	"encoding/json"
	"strings"
	"testing"

	"context"
	"github.com/qninq/sillyGirlPro/core/common"
	"github.com/qninq/sillyGirlPro/proto3/srpc"
	"github.com/qninq/sillyGirlPro/utils"
	"google.golang.org/grpc/metadata"
)

func TestPluginRuntimeUserList(t *testing.T) {
	originalFunctions := Functions
	t.Cleanup(func() { Functions = originalFunctions })

	plugin := &common.Function{UUID: "authorization-plugin", Title: "授权插件", Type: NODE, Open: true, HasUserForm: true}
	Functions = []*common.Function{plugin}
	user := &normalUser{
		ID:        "authorization-user-id",
		Username:  "authorization-user",
		Nickname:  "授权用户",
		CreatedAt: 1,
	}
	if _, _, err := userBucket.Set(normalUserStorageKey(user.Username), utils.JsonMarshal(user)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _, _ = userBucket.Set(normalUserStorageKey(user.Username), nil)
		_, _, _ = userBucket.Set(normalUserBindingsStorageKey(user.Username), nil)
	})
	bindings := normalUserBindings{
		QQ:       "10001",
		Telegram: "20002",
	}
	if _, _, err := userBucket.Set(normalUserBindingsStorageKey(user.Username), utils.JsonMarshal(bindings)); err != nil {
		t.Fatal(err)
	}

	users := pluginRuntimeUsers(plugin.UUID)
	if len(users) != 1 {
		t.Fatalf("runtime user count = %d", len(users))
	}
	gotUser := users[0]
	if !gotUser.Authorized || gotUser.Disabled || gotUser.Bindings.QQ != "10001" || gotUser.Bindings.Telegram != "20002" {
		t.Fatalf("unexpected runtime user: %#v", gotUser)
	}
	userValue := pluginUserRuntimeValue(plugin.UUID, pluginUserRuntimeListKey)
	if !strings.HasPrefix(userValue, "o:[") {
		t.Fatalf("runtime user list is not an array value: %q", userValue)
	}
	runtimeID := "runtime-user-list-test"
	registerRuntimePlugin(runtimeID, plugin.UUID)
	t.Cleanup(func() { deleteSenderRegister(runtimeID) })
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("runtime_id", runtimeID))
	response, err := (&SillyGirlService{}).BucketGet(ctx, &srpc.BucketKeyRequest{
		Name: pluginUserRuntimeBucket,
		Key:  pluginUserRuntimeListKey,
	})
	if err != nil {
		t.Fatal(err)
	}
	grpcUsers := []pluginRuntimeUser{}
	if err := json.Unmarshal([]byte(strings.TrimPrefix(response.Value, "o:")), &grpcUsers); err != nil {
		t.Fatal(err)
	}
	if len(grpcUsers) != 1 || !grpcUsers[0].Authorized {
		t.Fatalf("gRPC runtime user view = %#v", grpcUsers)
	}
	service := &SillyGirlService{}
	if _, err := service.BucketSet(ctx, &srpc.BucketSetRequest{Name: pluginUserRuntimeBucket, Key: pluginUserRuntimeListKey, Value: "o:[]"}); err == nil {
		t.Fatal("runtime user view accepted a write")
	}
	keys, err := service.BucketKeys(ctx, &srpc.BucketRequest{Name: pluginUserRuntimeBucket})
	if err != nil || len(keys.Keys) != 1 || keys.Keys[0] != pluginUserRuntimeListKey {
		t.Fatalf("runtime user view keys = %#v, err = %v", keys, err)
	}
	length, err := service.BucketLen(ctx, &srpc.BucketRequest{Name: pluginUserRuntimeBucket})
	if err != nil || length.Length != 1 {
		t.Fatalf("runtime user view length = %#v, err = %v", length, err)
	}

	plugin.Open = false
	users = pluginRuntimeUsers(plugin.UUID)
	if len(users) != 1 || users[0].Authorized {
		t.Fatalf("closed plugin retained effective authorization: %#v", users)
	}
	plugin.Open = true
	plugin.Status = pluginStatusValue(false)
	users = pluginRuntimeUsers(plugin.UUID)
	if len(users) != 1 || users[0].Authorized {
		t.Fatalf("status=false plugin retained effective authorization: %#v", users)
	}
	plugin.Status = pluginStatusValue(true)
	plugin.HasUserForm = false
	users = pluginRuntimeUsers(plugin.UUID)
	if len(users) != 1 || users[0].Authorized {
		t.Fatalf("form-less plugin retained effective authorization: %#v", users)
	}
	plugin.HasUserForm = true
	user.Disabled = true
	if _, _, err := userBucket.Set(normalUserStorageKey(user.Username), utils.JsonMarshal(user)); err != nil {
		t.Fatal(err)
	}
	users = pluginRuntimeUsers(plugin.UUID)
	if len(users) != 1 || users[0].Authorized {
		t.Fatalf("disabled user retained effective authorization: %#v", users)
	}
}

func TestPluginRuntimeUsersRejectUnknownPlugin(t *testing.T) {
	if users := pluginRuntimeUsers(""); len(users) != 0 {
		t.Fatalf("empty plugin runtime saw users: %#v", users)
	}
	if users := pluginRuntimeUsers("missing-plugin"); len(users) != 0 {
		t.Fatalf("unknown plugin runtime saw users: %#v", users)
	}
}

func TestPluginIDBoundToRuntimeContext(t *testing.T) {
	registerRuntimePlugin("runtime-authorization-test", "plugin-authorization-test")
	t.Cleanup(func() { deleteSenderRegister("runtime-authorization-test") })
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("runtime_id", "runtime-authorization-test"))
	if got := pluginIDFromRuntimeContext(ctx); got != "plugin-authorization-test" {
		t.Fatalf("pluginIDFromRuntimeContext = %q", got)
	}
}
