// Copyright 2017 Google Inc. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

package kms

import (
	"bytes"
	"context"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"cloud.google.com/go/iam"
	cloudkms "cloud.google.com/go/kms/apiv1"
	"github.com/GoogleCloudPlatform/golang-samples/internal/testutil"
	kmspb "google.golang.org/genproto/googleapis/cloud/kms/v1"
)

type TestVariables struct {
	ctx            context.Context
	projectId      string
	message        string
	location       string
	parent         string
	member         string
	role           iam.RoleName
	keyRing        string
	keyRingPath    string
	symPath        string
	symVersionPath string
	rsaDecryptPath string
	rsaSignPath    string
	ecSignPath     string
	symId          string
	rsaDecryptId   string
	rsaSignId      string
	ecSignId       string
	tryLimit       int
	waitTime       time.Duration
}

func getTestVariables(projectID string) TestVariables {
	var v TestVariables
	location := "global"
	parent := "projects/" + projectID + "/locations/" + location
	keyRing := "kms-samples"
	keyRingPath := parent + "/keyRings/" + keyRing

	symId := "symmetric"
	rsaDecryptId := "rsa-decrypt"
	rsaSignId := "rsa-sign"
	ecSignId := "ec-sign"

	sym := keyRingPath + "/cryptoKeys/" + symId
	symVersion := sym + "/cryptoKeyVersions/1"
	rsaDecrypt := keyRingPath + "/cryptoKeys/" + rsaDecryptId + "/cryptoKeyVersions/2"
	rsaSign := keyRingPath + "/cryptoKeys/" + rsaSignId + "/cryptoKeyVersions/1"
	ecSign := keyRingPath + "/cryptoKeys/" + ecSignId + "/cryptoKeyVersions/1"

	message := "test message 123"

	ctx := context.Background()

	member := "group:test@google.com"
	role := iam.Viewer

	tryLimit := 20
	waitTime := 5 * time.Second

	v = TestVariables{ctx, projectID, message, location, parent, member, role, keyRing, keyRingPath,
		sym, symVersion, rsaDecrypt, rsaSign, ecSign, symId, rsaDecryptId, rsaSignId, ecSignId, tryLimit, waitTime}
	return v
}

func createKeyHelper(v TestVariables, keyId, keyPath, parent string,
	purpose kmspb.CryptoKey_CryptoKeyPurpose, algorithm kmspb.CryptoKeyVersion_CryptoKeyVersionAlgorithm) bool {
	client, _ := cloudkms.NewKeyManagementClient(v.ctx)
	if _, err := getAsymmetricPublicKey(keyPath); err != nil {
		versionObj := &kmspb.CryptoKeyVersionTemplate{Algorithm: algorithm}
		keyObj := &kmspb.CryptoKey{Purpose: purpose, VersionTemplate: versionObj}

		client.CreateKeyRing(v.ctx, &kmspb.CreateKeyRingRequest{Parent: parent, KeyRingId: v.keyRing})
		client.CreateCryptoKey(v.ctx, &kmspb.CreateCryptoKeyRequest{Parent: parent + "/keyRings/" + v.keyRing, CryptoKeyId: keyId, CryptoKey: keyObj})
		return true
	}
	return false
}

func TestMain(m *testing.M) {
	tc, _ := testutil.ContextMain(m)
	v := getTestVariables(tc.ProjectID)
	parent := "projects/" + v.projectId + "/locations/global"
	// Create cryptokeys in the test project if needed.
	s1 := createKeyHelper(v, v.rsaDecryptId, v.rsaDecryptPath, parent, kmspb.CryptoKey_ASYMMETRIC_DECRYPT, kmspb.CryptoKeyVersion_RSA_DECRYPT_OAEP_2048_SHA256)
	s2 := createKeyHelper(v, v.rsaSignId, v.rsaSignPath, parent, kmspb.CryptoKey_ASYMMETRIC_SIGN, kmspb.CryptoKeyVersion_RSA_SIGN_PSS_2048_SHA256)
	s3 := createKeyHelper(v, v.ecSignId, v.ecSignPath, parent, kmspb.CryptoKey_ASYMMETRIC_SIGN, kmspb.CryptoKeyVersion_EC_SIGN_P256_SHA256)
	s4 := createKeyHelper(v, v.symId, v.symPath, parent, kmspb.CryptoKey_ENCRYPT_DECRYPT, kmspb.CryptoKeyVersion_GOOGLE_SYMMETRIC_ENCRYPTION)
	if s1 || s2 || s3 || s4 {
		// Leave time for keys to initialize.
		time.Sleep(30 * time.Second)
	}
	// Restore any disabled keys
	for _, keyPath := range []string{v.symVersionPath, v.rsaDecryptPath, v.ecSignPath} {
		restoreCryptoKeyVersion(keyPath)
		enableCryptoKeyVersion(keyPath)
	}
	// Run tests.
	exitCode := m.Run()
	os.Exit(exitCode)
}

func TestCreateKeyRing(t *testing.T) {
	t.Skip("TestCreateKeyRing skipped. There's currently no method to delete keyrings, so we should avoid creating resources")
	tc := testutil.SystemTest(t)
	v := getTestVariables(tc.ProjectID)

	ringId := v.keyRing + "testcreate"
	err := createKeyRing(v.parent, ringId)
	if err != nil {
		t.Fatalf("createKeyRing(%s, %s): %v", v.projectId, ringId, err)
	}
	client, _ := cloudkms.NewKeyManagementClient(v.ctx)
	resp, err := client.GetKeyRing(v.ctx, &kmspb.GetKeyRingRequest{Name: ringId})
	if err != nil {
		t.Fatalf("GetKeyRing(%s): %v", ringId, err)
	}
	if !strings.Contains(resp.Name, ringId) {
		t.Fatalf("new ring %s does not contain requested id %s: %v", resp.Name, ringId, err)
	}
}

func TestCreateCryptoKey(t *testing.T) {
	t.Skip("TestCreateCryptoKey skipped. There's currently no method to delete keys, so we should avoid creating resources")
	tc := testutil.SystemTest(t)
	v := getTestVariables(tc.ProjectID)

	keyId := "test-" + strconv.Itoa(int(time.Now().Unix()))
	err := createCryptoKey(v.keyRingPath, keyId)
	if err != nil {
		t.Fatalf("createKey(%s, %s): %v", v.keyRingPath, keyId, err)
	}
	client, _ := cloudkms.NewKeyManagementClient(v.ctx)
	keyPath := v.keyRingPath + "/cryptoKeys/" + keyId
	resp, err := client.GetCryptoKey(v.ctx, &kmspb.GetCryptoKeyRequest{Name: keyPath})
	if err != nil {
		t.Fatalf("GetCryptoKey(%s): %v", keyPath, err)
	}
	if !strings.Contains(resp.Name, keyId) {
		t.Fatalf("new key %s does not contain requested id %s: %v", resp.Name, keyId, err)
	}
	// mark for destruction
	destroyCryptoKeyVersion(keyPath + "/cryptoKeyVersions/1")
}

// tests disable/enable/destroy/restore
func TestChangeKeyVersionState(t *testing.T) {
	tc := testutil.SystemTest(t)
	v := getTestVariables(tc.ProjectID)
	client, _ := cloudkms.NewKeyManagementClient(v.ctx)

	for _, keyPath := range []string{v.symVersionPath, v.rsaDecryptPath, v.ecSignPath} {
		// test disable
		testutil.Retry(t, v.tryLimit, v.waitTime, func(r *testutil.R) {
			if err := disableCryptoKeyVersion(keyPath); err != nil {
				r.Errorf("disableCryptoKeyVersion(%s): %v", keyPath, err)
			}
			resp, err := client.GetCryptoKeyVersion(v.ctx, &kmspb.GetCryptoKeyVersionRequest{Name: keyPath})
			if err != nil {
				r.Errorf("GetCryptoKeyVersion(%s): %v", keyPath, err)
			}
			if resp.State != kmspb.CryptoKeyVersion_DISABLED {
				r.Errorf("failed to disable cryptokey version. current state: %v", resp.State)
			}
		})
		// test destroy
		testutil.Retry(t, v.tryLimit, v.waitTime, func(r *testutil.R) {
			if err := destroyCryptoKeyVersion(keyPath); err != nil {
				r.Errorf("destroyCryptoKeyVersion(%s): %v", keyPath, err)
			}
			resp, err := client.GetCryptoKeyVersion(v.ctx, &kmspb.GetCryptoKeyVersionRequest{Name: keyPath})
			if err != nil {
				r.Errorf("GetCryptoKeyVersion(%s): %v", keyPath, err)
			}
			if resp.State != kmspb.CryptoKeyVersion_DESTROY_SCHEDULED {
				r.Errorf("failed to destroy cryptokey version. current state: %v", resp.State)
			}
		})
		// test restore
		testutil.Retry(t, v.tryLimit, v.waitTime, func(r *testutil.R) {
			if err := restoreCryptoKeyVersion(keyPath); err != nil {
				r.Errorf("restoreCryptoKeyVersion(%s): %v", keyPath, err)
			}
			resp, err := client.GetCryptoKeyVersion(v.ctx, &kmspb.GetCryptoKeyVersionRequest{Name: keyPath})
			if err != nil {
				r.Errorf("GetCryptoKeyVersion(%s): %v", keyPath, err)
			}
			if resp.State != kmspb.CryptoKeyVersion_DISABLED {
				r.Errorf("failed to restore cryptokey version. current state: %v", resp.State)
			}
		})
		// test re-enable
		testutil.Retry(t, v.tryLimit, v.waitTime, func(r *testutil.R) {
			if err := enableCryptoKeyVersion(keyPath); err != nil {
				r.Errorf("enableCryptoKeyVersion(%s): %v", keyPath, err)
			}
			resp, err := client.GetCryptoKeyVersion(v.ctx, &kmspb.GetCryptoKeyVersionRequest{Name: keyPath})
			if err != nil {
				r.Errorf("GetCryptoKeyVersion(%s): %v", keyPath, err)
			}
			if resp.State != kmspb.CryptoKeyVersion_ENABLED {
				r.Errorf("failed to enable cryptokey version. current state: %v", resp.State)
			}
		})
	}
}

func TestGetRingPolicy(t *testing.T) {
	tc := testutil.SystemTest(t)
	v := getTestVariables(tc.ProjectID)

	policy, err := getRingPolicy(v.keyRingPath)
	if err != nil {
		t.Fatalf("GetRingPolicy(%s): %v", v.keyRingPath, err)
	}
	if policy == nil {
		t.Fatalf("GetRingPolicy(%s) returned nil policy", v.keyRingPath)
	}
}

func TestAddMemberRingPolicy(t *testing.T) {
	tc := testutil.SystemTest(t)
	v := getTestVariables(tc.ProjectID)

	// add member
	testutil.Retry(t, v.tryLimit, v.waitTime, func(r *testutil.R) {
		if err := addMemberRingPolicy(v.keyRingPath, v.member, v.role); err != nil {
			r.Errorf("addMemberRingPolicy(%s, %s, %s): %v", v.keyRingPath, v.member, v.role, err)
		}
	})
	policy, _ := getRingPolicy(v.keyRingPath)
	found := false
	for _, m := range policy.Members(v.role) {
		if m == v.member {
			found = true
		}
	}
	if found == false {
		t.Fatalf("could not find member '%s' for role '%s'", v.member, v.role)
	}
	// remove member
	testutil.Retry(t, v.tryLimit, v.waitTime, func(r *testutil.R) {
		if err := removeMemberRingPolicy(v.keyRingPath, v.member, v.role); err != nil {
			r.Errorf("removeMemberCryptoKeyPolicy(%s, %s, %s): %v", v.symPath, v.member, v.role, err)
		}
	})
	policy, _ = getRingPolicy(v.keyRingPath)
	found = false
	for _, m := range policy.Members(v.role) {
		if m == v.member {
			found = true
		}
	}
	if found == true {
		t.Fatalf("member '%s' found after attempted delete", v.member)
	}
}

func TestAddRemoveMemberCryptoKey(t *testing.T) {
	tc := testutil.SystemTest(t)
	v := getTestVariables(tc.ProjectID)

	rsaPath := v.keyRingPath + "/cryptoKeys/" + v.rsaDecryptId
	ecPath := v.keyRingPath + "/cryptoKeys/" + v.ecSignId
	for _, keyPath := range []string{v.symPath, rsaPath, ecPath} {
		// add member
		testutil.Retry(t, v.tryLimit, v.waitTime, func(r *testutil.R) {
			if err := addMemberCryptoKeyPolicy(keyPath, v.member, v.role); err != nil {
				r.Errorf("addMemberCryptoKeyPolicy(%s, %s, %s): %v", keyPath, v.member, v.role, err)
			}
		})
		policy, _ := getCryptoKeyPolicy(keyPath)
		found := false
		for _, m := range policy.Members(v.role) {
			if m == v.member {
				found = true
			}
		}
		if found == false {
			t.Fatalf("could not find member '%s' for role '%s' in key: %s", v.member, v.role, keyPath)
		}
		// remove member
		testutil.Retry(t, v.tryLimit, v.waitTime, func(r *testutil.R) {
			if err := removeMemberCryptoKeyPolicy(keyPath, v.member, v.role); err != nil {
				r.Errorf("removeMemberCryptoKeyPolicy(%s, %s, %s): %v", keyPath, v.member, v.role, err)
			}
		})
		policy, _ = getCryptoKeyPolicy(keyPath)
		found = false
		for _, m := range policy.Members(v.role) {
			if m == v.member {
				found = true
			}
		}
		if found != false {
			t.Fatalf("member '%s' found in key %s after attempted delete", v.member, keyPath)
		}
	}
}

func TestSymmetricEncryptDecrypt(t *testing.T) {
	tc := testutil.SystemTest(t)
	v := getTestVariables(tc.ProjectID)

	cipherBytes, err := encryptSymmetric(v.symPath, []byte(v.message))
	if err != nil {
		t.Fatalf("encrypt(%s, %s): %v", v.symPath, []byte(v.message), err)
	}
	plainBytes, err := decryptSymmetric(v.symPath, cipherBytes)
	if err != nil {
		t.Fatalf("decrypt(%s, %s): %v", v.symPath, cipherBytes, err)
	}
	if !bytes.Equal(plainBytes, []byte(v.message)) {
		t.Fatalf("decrypted plaintext does not match input message: want %s, got %s", []byte(v.message), plainBytes)
	}
	if bytes.Equal(cipherBytes, []byte(v.message)) {
		t.Fatalf("ciphertext and plaintext bytes are identical: %s", cipherBytes)
	}
	plaintext := string(plainBytes)
	if plaintext != v.message {
		t.Fatalf("failed to decypt expected plaintext: want %s, got %s", v.message, plaintext)
	}
}
