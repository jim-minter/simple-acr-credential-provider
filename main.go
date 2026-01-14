package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	kjson "k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/kubelet/pkg/apis/credentialprovider/install"
	v1 "k8s.io/kubelet/pkg/apis/credentialprovider/v1"
)

var (
	scheme = runtime.NewScheme()
	s      = kjson.NewSerializer(kjson.DefaultMetaFactory, scheme, scheme, false)
	codec  = serializer.NewCodecFactory(scheme).CodecForVersions(s, s, schema.GroupVersions{v1.SchemeGroupVersion}, schema.GroupVersions{v1.SchemeGroupVersion})

	// https://learn.microsoft.com/en-us/azure/azure-resource-manager/management/resource-name-rules#microsoftcontainerregistry
	acr = regexp.MustCompile(`(?i)^([a-z]{5,50}\.azurecr\.io)/`)
)

func init() {
	install.Install(scheme)
}

func main() {
	if err := run(context.Background()); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context) error {
	b, err := io.ReadAll(os.Stdin)
	if err != nil {
		return err
	}

	req := &v1.CredentialProviderRequest{}
	if _, _, err = codec.Decode(b, nil, req); err != nil {
		return err
	}

	resp, err := handle(ctx, req)
	if err != nil {
		log.Print(err)
		resp = &v1.CredentialProviderResponse{}
	}

	return codec.Encode(resp, os.Stdout)
}

func handle(ctx context.Context, req *v1.CredentialProviderRequest) (*v1.CredentialProviderResponse, error) {
	m := acr.FindStringSubmatch(req.Image)
	if m == nil {
		return &v1.CredentialProviderResponse{}, nil
	}

	credentials, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return nil, err
	}

	token, err := credentials.GetToken(ctx, policy.TokenRequestOptions{
		Scopes: []string{"https://containerregistry.azure.net/.default"},
	})
	if err != nil {
		return nil, err
	}

	// https://azure.github.io/acr/AAD-OAuth.html#calling-post-oauth2-exchange-to-get-an-acr-refresh-token
	resp, err := http.PostForm("https://"+m[1]+"/oauth2/exchange", url.Values{
		"grant_type":   []string{"access_token"},
		"service":      []string{m[1]},
		"tenant":       []string{os.Getenv("AZURE_TENANT_ID")},
		"access_token": []string{token.Token},
	})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code %d", resp.StatusCode)
	}

	switch resp.Header.Get("Content-Type") {
	case "application/json", "application/json; charset=utf-8":
	default:
		return nil, fmt.Errorf("unexpected content-type %q", resp.Header.Get("Content-Type"))
	}

	var payload struct {
		RefreshToken string `json:"refresh_token"`
	}

	if err = json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}

	return &v1.CredentialProviderResponse{
		CacheKeyType: v1.RegistryPluginCacheKeyType,
		CacheDuration: &metav1.Duration{
			Duration: 5 * time.Minute,
		},
		Auth: map[string]v1.AuthConfig{
			m[1]: {
				Username: "00000000-0000-0000-0000-000000000000",
				Password: payload.RefreshToken,
			},
		},
	}, nil
}
