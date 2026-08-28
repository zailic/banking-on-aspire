package keycloak

import (
	"context"
	"errors"
	"strings"

	authv1 "dev.local/banking-on-aspire/platform/gen/go/banking/auth/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
)

type Verifier interface {
	Verify(context.Context, string) (*Claims, error)
}

type claimsContextKey struct{}

// UnaryServerInterceptor authenticates callers and enforces the authorization
// option declared on each protobuf RPC. RPCs without a policy are denied.
func UnaryServerInterceptor(verifier Verifier, clientID string) grpc.UnaryServerInterceptor {
	if verifier == nil {
		panic("Keycloak token verifier is required")
	}
	if strings.TrimSpace(clientID) == "" {
		panic("Keycloak client ID is required")
	}
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if strings.HasPrefix(info.FullMethod, "/grpc.health.v1.Health/") {
			return handler(ctx, req)
		}
		policy, err := authorizationPolicy(info.FullMethod)
		if err != nil {
			return nil, status.Error(codes.Internal, "RPC authorization policy could not be resolved")
		}
		if policy == nil {
			return nil, status.Error(codes.PermissionDenied, "RPC access is not configured")
		}
		if policy.GetAllowUnauthenticated() {
			return handler(ctx, req)
		}
		token, err := bearerToken(ctx)
		if err != nil {
			return nil, status.Error(codes.Unauthenticated, "a valid bearer token is required")
		}
		claims, err := verifier.Verify(ctx, token)
		if err != nil {
			return nil, status.Error(codes.Unauthenticated, "bearer token validation failed")
		}
		if !hasRequiredPermissions(claims, clientID, policy.GetRequiredPermissions()) {
			return nil, status.Error(codes.PermissionDenied, "the caller does not have the required permissions")
		}
		return handler(WithClaims(ctx, claims), req)
	}
}

func authorizationPolicy(fullMethod string) (*authv1.AuthorizationPolicy, error) {
	name := strings.ReplaceAll(strings.TrimPrefix(fullMethod, "/"), "/", ".")
	descriptor, err := protoregistry.GlobalFiles.FindDescriptorByName(protoreflect.FullName(name))
	if err != nil {
		return nil, err
	}
	method, ok := descriptor.(protoreflect.MethodDescriptor)
	if !ok {
		return nil, errors.New("resolved descriptor is not an RPC method")
	}
	options, ok := method.Options().(*descriptorpb.MethodOptions)
	if !ok {
		return nil, errors.New("RPC method options have an unexpected type")
	}
	if !proto.HasExtension(options, authv1.E_Authorization) {
		return nil, nil
	}
	policy, ok := proto.GetExtension(options, authv1.E_Authorization).(*authv1.AuthorizationPolicy)
	if !ok {
		return nil, errors.New("RPC authorization policy has an unexpected type")
	}
	return policy, nil
}

func hasRequiredPermissions(claims *Claims, clientID string, requiredPermissions []string) bool {
	for _, permission := range requiredPermissions {
		if strings.TrimSpace(permission) == "" || !claims.HasClientRole(clientID, permission) {
			return false
		}
	}
	return true
}

func bearerToken(ctx context.Context) (string, error) {
	values := metadata.ValueFromIncomingContext(ctx, "authorization")
	if len(values) != 1 || !strings.HasPrefix(values[0], "Bearer ") {
		return "", errors.New("authorization metadata is missing")
	}
	token := strings.TrimSpace(strings.TrimPrefix(values[0], "Bearer "))
	if token == "" {
		return "", errors.New("bearer token is empty")
	}
	return token, nil
}

func WithClaims(ctx context.Context, claims *Claims) context.Context {
	return context.WithValue(ctx, claimsContextKey{}, claims)
}

func ClaimsFromContext(ctx context.Context) (*Claims, bool) {
	claims, ok := ctx.Value(claimsContextKey{}).(*Claims)
	return claims, ok && claims != nil
}
