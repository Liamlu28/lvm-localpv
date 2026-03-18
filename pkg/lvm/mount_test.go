package lvm

import (
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"

	mount "k8s.io/mount-utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	apis "github.com/openebs/lvm-localpv/pkg/apis/openebs.io/lvm/v1alpha1"
)

func TestUniqueMounts(t *testing.T) {
	input := []string{"/a", "/a", "/b", "/a", "/b", "/c"}
	want := []string{"/a", "/b", "/c"}

	got := uniqueMounts(input)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("uniqueMounts() = %v, want %v", got, want)
	}
}

func TestUmountVolumeReturnsErrorWhenMountStillPresent(t *testing.T) {
	restore := stubMountHelpers()
	defer restore()

	targetPath := "/var/lib/kubelet/pods/old-pod/volumes/kubernetes.io~csi/pvc-test/mount"
	devicePath := "/dev/mapper/vg_data-pvc--test"

	newSafeFormatAndMount = func() *mount.SafeFormatAndMount {
		return &mount.SafeFormatAndMount{
			Interface: mount.NewFakeMounter([]mount.MountPoint{{Device: devicePath, Path: targetPath}}),
		}
	}
	getDeviceNameFromMount = func(mounter mount.Interface, mountPath string) (string, int, error) {
		return devicePath, 1, nil
	}
	pathExistsFunc = func(path string) (bool, error) { return true, nil }
	getDeviceMounts = func(path string) ([]string, error) {
		return []string{targetPath, targetPath}, nil
	}

	removeCalled := false
	removePath = func(path string) error {
		removeCalled = true
		return nil
	}

	err := UmountVolume(testVolume("pvc-test", "vg_data"), targetPath)
	if err == nil {
		t.Fatal("expected UmountVolume to fail when stale mount remains")
	}
	if !strings.Contains(err.Error(), "unmount incomplete") {
		t.Fatalf("expected stale-mount error, got %v", err)
	}
	if removeCalled {
		t.Fatal("removePath should not be called when stale mount is still present")
	}
}

func TestUmountVolumeReturnsErrorWhenRemoveFails(t *testing.T) {
	restore := stubMountHelpers()
	defer restore()

	targetPath := "/var/lib/kubelet/pods/old-pod/volumes/kubernetes.io~csi/pvc-test/mount"
	devicePath := "/dev/mapper/vg_data-pvc--test"

	newSafeFormatAndMount = func() *mount.SafeFormatAndMount {
		return &mount.SafeFormatAndMount{
			Interface: mount.NewFakeMounter([]mount.MountPoint{{Device: devicePath, Path: targetPath}}),
		}
	}
	getDeviceNameFromMount = func(mounter mount.Interface, mountPath string) (string, int, error) {
		return devicePath, 1, nil
	}
	pathExistsFunc = func(path string) (bool, error) { return true, nil }
	getDeviceMounts = func(path string) ([]string, error) { return nil, nil }
	removePath = func(path string) error { return errors.New("device or resource busy") }

	err := UmountVolume(testVolume("pvc-test", "vg_data"), targetPath)
	if err == nil {
		t.Fatal("expected UmountVolume to fail when removePath fails")
	}
	if !strings.Contains(err.Error(), "device or resource busy") {
		t.Fatalf("expected removePath error, got %v", err)
	}
}

func stubMountHelpers() func() {
	origNewSafeFormatAndMount := newSafeFormatAndMount
	origGetDeviceNameFromMount := getDeviceNameFromMount
	origPathExistsFunc := pathExistsFunc
	origRemovePath := removePath
	origGetDeviceMounts := getDeviceMounts

	return func() {
		newSafeFormatAndMount = origNewSafeFormatAndMount
		getDeviceNameFromMount = origGetDeviceNameFromMount
		pathExistsFunc = origPathExistsFunc
		removePath = origRemovePath
		getDeviceMounts = origGetDeviceMounts
	}
}

func testVolume(name, vg string) *apis.LVMVolume {
	return &apis.LVMVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name:       name,
			Finalizers: []string{"openebs.io/pvc-protection"},
		},
		Spec: apis.VolumeInfo{
			OwnerNodeID: NodeID,
			VolGroup:    vg,
			Capacity:    "1073741824",
		},
	}
}

func TestUmountVolumeIgnoresMissingPathAfterUnmount(t *testing.T) {
	restore := stubMountHelpers()
	defer restore()

	targetPath := "/var/lib/kubelet/pods/old-pod/volumes/kubernetes.io~csi/pvc-test/mount"
	devicePath := "/dev/mapper/vg_data-pvc--test"

	newSafeFormatAndMount = func() *mount.SafeFormatAndMount {
		return &mount.SafeFormatAndMount{
			Interface: mount.NewFakeMounter([]mount.MountPoint{{Device: devicePath, Path: targetPath}}),
		}
	}
	getDeviceNameFromMount = func(mounter mount.Interface, mountPath string) (string, int, error) {
		return devicePath, 1, nil
	}
	pathExistsFunc = func(path string) (bool, error) { return true, nil }
	getDeviceMounts = func(path string) ([]string, error) { return nil, nil }
	removePath = func(path string) error { return os.ErrNotExist }

	if err := UmountVolume(testVolume("pvc-test", "vg_data"), targetPath); err != nil {
		t.Fatalf("expected missing target path to be tolerated, got %v", err)
	}
}
