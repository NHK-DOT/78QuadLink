package main

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

func TestA1JointContract(t *testing.T) {
	b, err := os.ReadFile("../../testdata/a1.urdf")
	if err != nil {
		t.Fatal(err)
	}
	var tree xmlNode
	if err = xml.Unmarshal(b, &tree); err != nil {
		t.Fatal(err)
	}
	joints, err := checkJoints(tree)
	if err != nil {
		t.Fatal(err)
	}
	if len(joints) != 12 || joints[0].Name != "FR_hip_joint" || joints[11].Name != "RL_calf_joint" || joints[0].Effort != 33.5 {
		t.Fatalf("wrong contract: %+v", joints)
	}
	for i := range tree.Children {
		if tree.Children[i].XMLName.Local == "joint" && tree.Children[i].attr("name") == "FR_hip_joint" {
			tree.Children[i].Children = nil
			break
		}
	}
	if _, err = checkJoints(tree); err == nil {
		t.Fatal("accepted missing limit")
	}
}
func TestPreparationPreservesModelPhysics(t *testing.T) {
	repo := t.TempDir()
	model := filepath.Join(repo, "models/unitree_a1")
	os.MkdirAll(filepath.Join(model, "urdf"), 0755)
	fixture, err := os.ReadFile("../../testdata/a1.urdf")
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(model, "urdf/a1.urdf"), fixture, 0644)
	// Parser-only mesh placeholders; actual dynamics are tested in pinned integration.
	for _, m := range regexp.MustCompile(`package://a1_description/([^" ]+)`).FindAllStringSubmatch(string(fixture), -1) {
		target := filepath.Join(model, m[1])
		os.MkdirAll(filepath.Dir(target), 0755)
		os.WriteFile(target, []byte("fixture"), 0644)
	}
	config := filepath.Join(repo, "gazebo_ros2/go1_sim/go1_description/config/jtc.yaml")
	os.MkdirAll(filepath.Dir(config), 0755)
	os.WriteFile(config, []byte("# parser fixture\n"), 0644)
	directory := filepath.Join(t.TempDir(), "prepared")
	if err := prepareRobot([]string{"-repo", repo, "-output", directory}); err != nil {
		t.Fatal(err)
	}
	var before, after xmlNode
	b, _ := os.ReadFile("../../testdata/a1.urdf")
	a, _ := os.ReadFile(filepath.Join(directory, "robot.urdf"))
	xml.Unmarshal(b, &before)
	xml.Unmarshal(a, &after)
	lookup := func(tree xmlNode, name string) *xmlNode {
		for i := range tree.Children {
			if tree.Children[i].XMLName.Local == "link" && tree.Children[i].attr("name") == name {
				return &tree.Children[i]
			}
		}
		return nil
	}
	trunkBefore, trunkAfter := lookup(before, "trunk"), lookup(after, "trunk")
	if trunkBefore == nil || trunkAfter == nil {
		t.Fatal("trunk missing")
	}
	// Parse numeric model invariants, not serialized whitespace.
	for _, name := range []string{"mass", "inertia"} {
		l := trunkBefore.child("inertial").child(name)
		r := trunkAfter.child("inertial").child(name)
		for _, attr := range l.Attr {
			if r.attr(attr.Name.Local) != attr.Value {
				t.Fatal("inertia changed")
			}
		}
	}
	if lookup(after, "base_link") == nil {
		t.Fatal("root frame not adapted")
	}
	if err := prepareRobot([]string{"-repo", repo, "-output", directory}); err == nil {
		t.Fatal("overwrote prepared model")
	}
}

func TestGo2Contract(t *testing.T) {
	b, err := os.ReadFile("../../testdata/go2.urdf")
	if err != nil {
		t.Fatal(err)
	}
	var tree xmlNode
	if err = xml.Unmarshal(b, &tree); err != nil {
		t.Fatal(err)
	}
	joints, err := checkJoints(tree)
	if err != nil {
		t.Fatal(err)
	}
	if len(joints) != 12 || joints[0].Effort != 23.7 || joints[2].Effort != 45.43 {
		t.Fatalf("wrong Go2 limits: %+v", joints)
	}
}
