// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright (C) 2022 Canonical Ltd
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License version 3 as
 * published by the Free Software Foundation.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package builtin

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/snapcore/snapd/interfaces"
	"github.com/snapcore/snapd/interfaces/apparmor"
	"github.com/snapcore/snapd/interfaces/seccomp"
	apparmor_sandbox "github.com/snapcore/snapd/sandbox/apparmor"
	"github.com/snapcore/snapd/snap"
	"github.com/snapcore/snapd/strutil"
)

const posixMQSummary = `allows access to POSIX message queues`

// This interface is super-privileged
const posixMQBaseDeclarationSlots = `
  posix-mq:
    allow-installation: false
    deny-connection: true
    deny-auto-connection: true
`

// Paths can only be specified by the slot and a slot needs to exist for the
// plug to connect to, so the plug does not need to be marked super-privileged
const posixMQBaseDeclarationPlugs = `
  posix-mq:
    allow-installation: true
    allow-connection:
      slot-attributes:
        posix-mq: $PLUG(posix-mq)
    allow-auto-connection:
      slot-publisher-id:
        - $PLUG_PUBLISHER_ID
      slot-attributes:
        posix-mq: $PLUG(posix-mq)
`

const posixMQPermanentSlotSecComp = `
mq_open
mq_getsetattr
mq_unlink
mq_notify
mq_timedreceive
mq_timedsend
`

var posixMQPlugPermissions = []string{
	"open",
	"read",
	"write",
	"create",
	"delete",
	"setattr",
	"getattr",
}

var posixMQDefaultPlugPermissions = []string{
	"read",
	"write",
}

// Ensure that the name matches the criteria from the mq_overview man page:
//   Each message queue is identified by a name of the form /somename;
//   that is, a null-terminated string of up to NAME_MAX (i.e., 255)
//   characters consisting of an initial slash, followed by one or more
//   characters, none of which are slashes.
var posixMQNamePattern = regexp.MustCompile(`^/[^/]{1,255}$`)

type posixMQInterface struct {
	commonInterface
}

func (iface *posixMQInterface) StaticInfo() interfaces.StaticInfo {
	return interfaces.StaticInfo{
		Summary:              posixMQSummary,
		BaseDeclarationSlots: posixMQBaseDeclarationSlots,
		BaseDeclarationPlugs: posixMQBaseDeclarationPlugs,
	}
}

func (iface *posixMQInterface) Name() string {
	return "posix-mq"
}

func (iface *posixMQInterface) checkPosixMQAppArmorSupport() error {
	if apparmor_sandbox.ProbedLevel() == apparmor_sandbox.Unsupported {
		/* AppArmor is not supported at all; no need to add rules */
		return nil
	}

	features, err := apparmor_sandbox.ParserFeatures()
	if err != nil {
		return err
	}

	if !strutil.ListContains(features, "mqueue") {
		return fmt.Errorf("AppArmor does not support POSIX message queues - cannot setup or connect interfaces")
	}

	return nil
}

func (iface *posixMQInterface) isValidPermission(perm string) bool {
	for _, validPerm := range posixMQPlugPermissions {
		if perm == validPerm {
			return true
		}
	}
	return false
}

func (iface *posixMQInterface) validatePermissionList(perms []string, name string) error {
	for _, perm := range perms {
		if !iface.isValidPermission(perm) {
			return fmt.Errorf("posix-mq slot %s permission \"%s\" not valid, must be one of %v", name, perm, posixMQPlugPermissions)
		}
	}

	return nil
}

func (iface *posixMQInterface) getPermissions(attrs interfaces.Attrer, name string) ([]string, error) {
	var perms []string

	if err := attrs.Attr("permissions", &perms); err != nil {
		/* If the permissions have not been specified, use the defaults */
		perms = posixMQDefaultPlugPermissions
	}

	if err := iface.validatePermissionList(perms, name); err != nil {
		return nil, err
	}

	return perms, nil
}

func (iface *posixMQInterface) getPath(attrs interfaces.Attrer, name string) (string, error) {
	var path string

	if pathAttr, isSet := attrs.Lookup("path"); isSet {
		if pathStr, ok := pathAttr.(string); ok {
			path = pathStr
		} else {
			return "", fmt.Errorf(`posix-mq slot %s "path" attribute must be a string, not %v`, name, pathAttr)
		}
	} else {
		/* path defaults to name if unspecified */
		path = name
	}

	/* Path must begin with a / */
	if path[0] != '/' {
		path = "/" + path
	}

	if err := iface.validatePath(name, path); err != nil {
		return "", err
	}

	return path, nil

}

func (iface *posixMQInterface) validatePath(name, path string) error {
	if !posixMQNamePattern.MatchString(path) {
		return fmt.Errorf(`posix-mq slot %s "path" attribute must conform to the POSIX message queue name specifications (see "man mq_overview"): %v`, name, path)
	}

	if err := apparmor_sandbox.ValidateNoAppArmorRegexp(path); err != nil {
		return fmt.Errorf(`posix-mq slot %s "path" attribute is invalid: %v"`, name, path)
	}

	if !cleanSubPath(path) {
		return fmt.Errorf(`posix-mq slot %s "path" attribute is not a clean path: %v"`, name, path)
	}

	return nil
}

func (iface *posixMQInterface) BeforePreparePlug(plug *snap.PlugInfo) error {
	if err := iface.checkPosixMQAppArmorSupport(); err != nil {
		return err
	}

	/* Only ensure that the given permissions are valid, don't use them here */
	if _, err := iface.getPermissions(plug, plug.Name); err != nil {
		return err
	}

	return nil
}

func (iface *posixMQInterface) BeforePrepareSlot(slot *snap.SlotInfo) error {
	if err := iface.checkPosixMQAppArmorSupport(); err != nil {
		return err
	}

	/* Only ensure that the given permissions are valid, don't use them here */
	if _, err := iface.getPermissions(slot, slot.Name); err != nil {
		return err
	}

	/* Only ensure that the given path is valid, don't use it here */
	if _, err := iface.getPath(slot, slot.Name); err != nil {
		return err
	}

	return nil
}

func (iface *posixMQInterface) AppArmorPermanentSlot(spec *apparmor.Specification, slot *snap.SlotInfo) error {
	if !implicitSystemPermanentSlot(slot) {
		if path, err := iface.getPath(slot, slot.Name); err == nil {
			spec.AddSnippet(fmt.Sprintf(`# POSIX Message Queue management
mqueue (create delete getattr setattr read write) "%s",
`, path))
		} else {
			return err
		}
	}
	return nil
}

func (iface *posixMQInterface) AppArmorConnectedSlot(spec *apparmor.Specification, plug *interfaces.ConnectedPlug, slot *interfaces.ConnectedSlot) error {
	if path, err := iface.getPath(slot, slot.Name()); err == nil {
		spec.AddSnippet(fmt.Sprintf(`# POSIX Message Queue slot communication
mqueue (create delete getattr setattr) "%s",
`, path))
	} else {
		return err
	}
	return nil
}

func (iface *posixMQInterface) AppArmorConnectedPlug(spec *apparmor.Specification, plug *interfaces.ConnectedPlug, slot *interfaces.ConnectedSlot) error {
	if path, err := iface.getPath(slot, slot.Name()); err == nil {
		if perms, err := iface.getPermissions(slot, slot.Name()); err == nil {
			aaPerms := strings.Join(perms, " ")
			spec.AddSnippet(fmt.Sprintf(`# POSIX Message Queue plug communication
mqueue (%s) "%s",
`, aaPerms, path))
		} else {
			return err
		}
	} else {
		return err
	}
	return nil
}

func (iface *posixMQInterface) SecCompPermanentSlot(spec *seccomp.Specification, slot *snap.SlotInfo) error {
	spec.AddSnippet(posixMQPermanentSlotSecComp)
	return nil
}

func (iface *posixMQInterface) SecCompConnectedPlug(spec *seccomp.Specification, plug *interfaces.ConnectedPlug, slot *interfaces.ConnectedSlot) error {
	if perms, err := iface.getPermissions(slot, slot.Name()); err == nil {
		var syscalls = []string{
			// always allow this function
			"mq_open",
		}
		for _, perm := range perms {
			switch perm {
			case "read":
				syscalls = append(syscalls, "mq_timedreceive")
				syscalls = append(syscalls, "mq_notify")
				break
			case "write":
				syscalls = append(syscalls, "mq_timedsend")
				break
			case "delete":
				syscalls = append(syscalls, "mq_unlink")
				break
			case "setattr":
				fallthrough
			case "getattr":
				syscalls = append(syscalls, "mq_getsetattr")
				break
			default:
				continue // no syscall needed
			}
		}
		spec.AddSnippet(strings.Join(syscalls, "\n"))
	} else {
		return err
	}
	return nil
}

func init() {
	registerIface(&posixMQInterface{})
}
