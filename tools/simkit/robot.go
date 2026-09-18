package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"flag"
	"fmt"
	"html"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type xmlNode struct {
	XMLName  xml.Name
	Attr     []xml.Attr `xml:",any,attr"`
	Text     string     `xml:",chardata"`
	Children []xmlNode  `xml:",any"`
}

func (n xmlNode) attr(key string) string {
	for _, a := range n.Attr {
		if a.Name.Local == key {
			return a.Value
		}
	}
	return ""
}
func (n xmlNode) child(name string) *xmlNode {
	for i := range n.Children {
		if n.Children[i].XMLName.Local == name {
			return &n.Children[i]
		}
	}
	return nil
}
func jointOrder() []string {
	var names []string
	for _, leg := range []string{"FR", "FL", "RR", "RL"} {
		for _, joint := range []string{"hip", "thigh", "calf"} {
			names = append(names, leg+"_"+joint+"_joint")
		}
	}
	return names
}

type jointContract struct {
	Name     string  `json:"name"`
	Slot     int     `json:"wire_index"`
	Lower    float64 `json:"lower_rad"`
	Upper    float64 `json:"upper_rad"`
	Effort   float64 `json:"effort_nm"`
	Velocity float64 `json:"velocity_rad_s"`
}

func checkJoints(root xmlNode) ([]jointContract, error) {
	joints := map[string]xmlNode{}
	for _, n := range root.Children {
		if n.XMLName.Local == "joint" && n.attr("type") != "fixed" {
			name := n.attr("name")
			if _, ok := joints[name]; ok {
				return nil, fmt.Errorf("duplicate joint %s", name)
			}
			joints[name] = n
		}
	}
	if len(joints) != 12 {
		return nil, fmt.Errorf("fixed12 adapter requires exactly 12 moving joints, got %d", len(joints))
	}
	var contracts []jointContract
	for i, name := range jointOrder() {
		n, ok := joints[name]
		if !ok || n.attr("type") != "revolute" {
			return nil, fmt.Errorf("missing revolute joint %s", name)
		}
		limit := n.child("limit")
		if limit == nil {
			return nil, fmt.Errorf("missing limits: %s", name)
		}
		values := map[string]float64{}
		for _, key := range []string{"lower", "upper", "effort", "velocity"} {
			v, e := strconv.ParseFloat(limit.attr(key), 64)
			if e != nil || math.IsNaN(v) || math.IsInf(v, 0) {
				return nil, fmt.Errorf("invalid %s for %s", key, name)
			}
			values[key] = v
		}
		if values["lower"] >= values["upper"] || values["effort"] <= 0 || values["velocity"] <= 0 {
			return nil, fmt.Errorf("invalid range for %s", name)
		}
		contracts = append(contracts, jointContract{name, i, values["lower"], values["upper"], values["effort"], values["velocity"]})
	}
	return contracts, nil
}
func prepareRobot(args []string) error {
	fs := flag.NewFlagSet("prepare-robot", flag.ContinueOnError)
	root := fs.String("repo", ".", "78QuadLink source/deployment root")
	robot := fs.String("robot", "a1", "model adapter (a1, go2)")
	output := fs.String("output", "", "new output directory for generated URDF and contract")
	shared := fs.Bool("shared-imu", true, "include optional source IMU writer")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 || (*robot != "a1" && *robot != "go2") || *output == "" {
		return fmt.Errorf("prepare-robot requires -robot a1|go2 -output NEW_DIRECTORY")
	}
	repo, err := filepath.Abs(*root)
	if err != nil {
		return err
	}
	modelDir := filepath.Join(repo, "models/unitree_"+*robot)
	source := filepath.Join(modelDir, "urdf/"+*robot+".urdf")
	data, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	var tree xmlNode
	if err = xml.Unmarshal(data, &tree); err != nil {
		return err
	}
	if tree.XMLName.Local != "robot" {
		return fmt.Errorf("expected URDF robot")
	}
	joints, err := checkJoints(tree)
	if err != nil {
		return err
	}
	// Resolve/validate meshes and rename the root frame once, before simulation.
	assets := map[string]string{}
	var rewrite func(*xmlNode) error
	rewrite = func(n *xmlNode) error {
		n.Text = strings.TrimSpace(n.Text)
		for i, a := range n.Attr {
			if a.Value == "imu" && *robot == "go2" && (a.Name.Local == "name" || a.Name.Local == "link" || a.Name.Local == "reference") {
				n.Attr[i].Value = "imu_link"
			}
			if a.Value == "base" && (a.Name.Local == "name" || a.Name.Local == "link" || a.Name.Local == "reference") {
				n.Attr[i].Value = "base_link"
			}
			if a.Name.Local == "filename" && strings.HasPrefix(a.Value, "package://"+*robot+"_description/") {
				rel := strings.TrimPrefix(a.Value, "package://"+*robot+"_description/")
				path := filepath.Join(modelDir, rel)
				if !strings.HasPrefix(path, modelDir+string(os.PathSeparator)) {
					return fmt.Errorf("mesh outside model directory")
				}
				b, e := os.ReadFile(path)
				if e != nil {
					return e
				}
				sum := sha256.Sum256(b)
				assets[rel] = hex.EncodeToString(sum[:])
				n.Attr[i].Value = path
			}
		}
		for i := range n.Children {
			if e := rewrite(&n.Children[i]); e != nil {
				return e
			}
		}
		return nil
	}
	if err = rewrite(&tree); err != nil {
		return err
	}
	var keep []xmlNode
	for _, n := range tree.Children {
		if n.XMLName.Local == "transmission" {
			continue
		}
		if n.XMLName.Local == "gazebo" {
			var children []xmlNode
			for _, c := range n.Children {
				if c.XMLName.Local != "plugin" && c.XMLName.Local != "sensor" {
					children = append(children, c)
				}
			}
			n.Children = children
			if len(children) == 0 {
				continue
			}
		}
		keep = append(keep, n)
	}
	tree.Children = keep
	config := filepath.Join(repo, "gazebo_ros2/go1_sim/go1_description/config/jtc.yaml")
	if _, err = os.Stat(config); err != nil {
		return err
	}
	var extra strings.Builder
	extra.WriteString(`<robot><ros2_control name="GazeboSystem" type="system"><hardware><plugin>gazebo_ros2_control/GazeboSystem</plugin></hardware>`)
	for _, j := range joints {
		fmt.Fprintf(&extra, `<joint name="%s"><command_interface name="effort"><param name="min">%g</param><param name="max">%g</param></command_interface><state_interface name="position"/><state_interface name="velocity"/><state_interface name="effort"/></joint>`, j.Name, -j.Effort, j.Effort)
	}
	fmt.Fprintf(&extra, `</ros2_control><gazebo><plugin name="gazebo_ros2_control" filename="libgazebo_ros2_control.so"><robot_param>robot_description</robot_param><robot_param_node>robot_state_publisher_node</robot_param_node><parameters>%s</parameters></plugin></gazebo>`, html.EscapeString(config))
	extra.WriteString(`<gazebo reference="imu_link"><sensor name="imu_sensor" type="imu"><always_on>true</always_on><update_rate>1000</update_rate><plugin name="imu_plugin" filename="libgazebo_ros_imu_sensor.so"/>`)
	if *shared {
		extra.WriteString(`<plugin name="shared_imu" filename="libshared_imu.so"/>`)
	}
	extra.WriteString(`</sensor></gazebo><gazebo><plugin name="odom_plugin" filename="libgazebo_ros_p3d.so"><ros><namespace>/</namespace><remapping>odom:=/odom</remapping></ros><body_name>base_link</body_name><frame_name>odom</frame_name><update_rate>100</update_rate><gaussian_noise>0</gaussian_noise></plugin></gazebo></robot>`)
	var injected xmlNode
	if err = xml.Unmarshal([]byte(extra.String()), &injected); err != nil {
		return err
	}
	tree.Children = append(tree.Children, injected.Children...)
	encoded, err := xml.MarshalIndent(tree, "", "  ")
	if err != nil {
		return err
	}
	sum := sha256.Sum256(data)
	generated := sha256.Sum256(encoded)
	contract := map[string]interface{}{"robot": "Unitree " + strings.ToUpper(*robot), "entity": strings.ToUpper(*robot), "controller": "junior_ctrl_" + *robot, "wire_schema": "fixed12-v2", "board_abi": "go1sim-board-v1-8x1024", "source_sha256": hex.EncodeToString(sum[:]), "generated_sha256": hex.EncodeToString(generated[:]), "mesh_sha256": assets, "joints": joints, "shared_imu": *shared, "adaptation": "ROS1 plugins/transmissions removed; inertias, collisions and joint limits retained; ROS2 effort/IMU/odom adapters added at startup"}
	encodedContract, err := json.MarshalIndent(contract, "", "  ")
	if err != nil {
		return err
	}
	if err = os.Mkdir(*output, 0700); err != nil {
		return fmt.Errorf("output must be a new directory: %w", err)
	}
	if err = os.WriteFile(filepath.Join(*output, "robot.urdf"), encoded, 0600); err != nil {
		return err
	}
	if err = os.WriteFile(filepath.Join(*output, "contract.json"), encodedContract, 0600); err != nil {
		return err
	}
	return printJSON(contract)
}
