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

package builtin_test

import (
	. "gopkg.in/check.v1"

	"github.com/snapcore/snapd/interfaces"
	"github.com/snapcore/snapd/interfaces/apparmor"
	"github.com/snapcore/snapd/interfaces/builtin"
	"github.com/snapcore/snapd/interfaces/seccomp"
	apparmor_sandbox "github.com/snapcore/snapd/sandbox/apparmor"
	"github.com/snapcore/snapd/snap"
	"github.com/snapcore/snapd/snap/snaptest"
	"github.com/snapcore/snapd/testutil"
)

const slotSnapInfoYaml = `name: producer
version: 1.0

slots:
  test-rw:
    interface: posix-mq
    permissions:
      - read
      - write

  test-default:
    interface: posix-mq

  test-ro:
    interface: posix-mq
    permissions:
      - read

  test-all-perms:
    interface: posix-mq
    permissions:
      - create
      - delete
      - read
      - write

  test-invalid-path-1:
    interface: posix-mq
    posix-mq: ../../test-invalid

  test-invalid-path-2:
    interface: posix-mq
    posix-mq: /test-invalid-2"[

  test-invalid-path-3:
    interface: posix-mq
    posix-mq:
      - this-is-not-a-string

  test-invalid-perms-1:
    interface: posix-mq
    permissions:
      - create
      - delete
      - break-everything

  test-invalid-perms-2:
      interface: posix-mq
      permissions: not-a-list

apps:
  app:
    command: foo
    slots:
      - test-default-rw
      - test-rw
      - test-ro
      - test-all-perms
      - test-invalid-path-1
      - test-invalid-path-2
`

const defaultRWPlugSnapInfoYaml = `name: consumer
version: 1.0

plugs:
  test-default:
    interface: posix-mq

apps:
  app:
    command: foo
    plugs: [test-default]
`

const rwPlugSnapInfoYaml = `name: consumer
version: 1.0

plugs:
  test-rw:
    interface: posix-mq

apps:
  app:
    command: foo
    plugs: [test-rw]
`

const roPlugSnapInfoYaml = `name: consumer
version: 1.0

plugs:
  test-ro:
    interface: posix-mq

apps:
  app:
    command: foo
    plugs: [test-ro]
`

const allPermsPlugSnapInfoYaml = `name: consumer
version: 1.0

plugs:
  test-all-perms:
    interface: posix-mq

apps:
  app:
    command: foo
    plugs: [test-all-perms]
`

const invalidPerms1PlugSnapInfoYaml = `name: consumer
version: 1.0

plugs:
  test-invalid-perms-1:
    interface: posix-mq

apps:
  app:
    command: foo
    plugs: [test-invalid-perms-1]
`

type PosixMQInterfaceSuite struct {
	testutil.BaseTest

	iface interfaces.Interface

	testReadWriteSlotInfo *snap.SlotInfo
	testReadWriteSlot     *interfaces.ConnectedSlot
	testReadWritePlugInfo *snap.PlugInfo
	testReadWritePlug     *interfaces.ConnectedPlug

	testDefaultPermsSlotInfo *snap.SlotInfo
	testDefaultPermsSlot     *interfaces.ConnectedSlot
	testDefaultPermsPlugInfo *snap.PlugInfo
	testDefaultPermsPlug     *interfaces.ConnectedPlug

	testReadOnlySlotInfo *snap.SlotInfo
	testReadOnlySlot     *interfaces.ConnectedSlot
	testReadOnlyPlugInfo *snap.PlugInfo
	testReadOnlyPlug     *interfaces.ConnectedPlug

	testAllPermsSlotInfo *snap.SlotInfo
	testAllPermsSlot     *interfaces.ConnectedSlot
	testAllPermsPlugInfo *snap.PlugInfo
	testAllPermsPlug     *interfaces.ConnectedPlug

	testInvalidPath1SlotInfo *snap.SlotInfo
	testInvalidPath1Slot     *interfaces.ConnectedSlot

	testInvalidPath2SlotInfo *snap.SlotInfo
	testInvalidPath2Slot     *interfaces.ConnectedSlot

	testInvalidPath3SlotInfo *snap.SlotInfo
	testInvalidPath3Slot     *interfaces.ConnectedSlot

	testInvalidPerms1SlotInfo *snap.SlotInfo
	testInvalidPerms1Slot     *interfaces.ConnectedSlot
	testInvalidPerms1PlugInfo *snap.PlugInfo
	testInvalidPerms1Plug     *interfaces.ConnectedPlug

	testInvalidPerms2SlotInfo *snap.SlotInfo
	testInvalidPerms2Slot     *interfaces.ConnectedSlot
}

var _ = Suite(&PosixMQInterfaceSuite{
	iface: builtin.MustInterface("posix-mq"),
})

func (s *PosixMQInterfaceSuite) SetUpTest(c *C) {
	s.BaseTest.SetUpTest(c)

	slotSnap := snaptest.MockInfo(c, slotSnapInfoYaml, nil)

	s.testReadWriteSlotInfo = slotSnap.Slots["test-rw"]
	s.testReadWriteSlot = interfaces.NewConnectedSlot(s.testReadWriteSlotInfo, nil, nil)

	s.testDefaultPermsSlotInfo = slotSnap.Slots["test-default"]
	s.testDefaultPermsSlot = interfaces.NewConnectedSlot(s.testDefaultPermsSlotInfo, nil, nil)

	s.testReadOnlySlotInfo = slotSnap.Slots["test-ro"]
	s.testReadOnlySlot = interfaces.NewConnectedSlot(s.testReadOnlySlotInfo, nil, nil)

	s.testAllPermsSlotInfo = slotSnap.Slots["test-all-perms"]
	s.testAllPermsSlot = interfaces.NewConnectedSlot(s.testAllPermsSlotInfo, nil, nil)

	s.testInvalidPath1SlotInfo = slotSnap.Slots["test-invalid-path-1"]
	s.testInvalidPath1Slot = interfaces.NewConnectedSlot(s.testInvalidPath1SlotInfo, nil, nil)

	s.testInvalidPath2SlotInfo = slotSnap.Slots["test-invalid-path-2"]
	s.testInvalidPath2Slot = interfaces.NewConnectedSlot(s.testInvalidPath2SlotInfo, nil, nil)

	s.testInvalidPath3SlotInfo = slotSnap.Slots["test-invalid-path-3"]
	s.testInvalidPath3Slot = interfaces.NewConnectedSlot(s.testInvalidPath3SlotInfo, nil, nil)

	s.testInvalidPerms1SlotInfo = slotSnap.Slots["test-invalid-perms-1"]
	s.testInvalidPerms1Slot = interfaces.NewConnectedSlot(s.testInvalidPerms1SlotInfo, nil, nil)

	s.testInvalidPerms2SlotInfo = slotSnap.Slots["test-invalid-perms-2"]
	s.testInvalidPerms2Slot = interfaces.NewConnectedSlot(s.testInvalidPerms2SlotInfo, nil, nil)

	plugSnap0 := snaptest.MockInfo(c, rwPlugSnapInfoYaml, nil)
	s.testReadWritePlugInfo = plugSnap0.Plugs["test-rw"]
	s.testReadWritePlug = interfaces.NewConnectedPlug(s.testReadWritePlugInfo, nil, nil)

	plugSnap1 := snaptest.MockInfo(c, defaultRWPlugSnapInfoYaml, nil)
	s.testDefaultPermsPlugInfo = plugSnap1.Plugs["test-default"]
	s.testDefaultPermsPlug = interfaces.NewConnectedPlug(s.testDefaultPermsPlugInfo, nil, nil)

	plugSnap2 := snaptest.MockInfo(c, roPlugSnapInfoYaml, nil)
	s.testReadOnlyPlugInfo = plugSnap2.Plugs["test-ro"]
	s.testReadOnlyPlug = interfaces.NewConnectedPlug(s.testReadOnlyPlugInfo, nil, nil)

	plugSnap3 := snaptest.MockInfo(c, allPermsPlugSnapInfoYaml, nil)
	s.testAllPermsPlugInfo = plugSnap3.Plugs["test-all-perms"]
	s.testAllPermsPlug = interfaces.NewConnectedPlug(s.testAllPermsPlugInfo, nil, nil)

	plugSnap4 := snaptest.MockInfo(c, invalidPerms1PlugSnapInfoYaml, nil)
	s.testInvalidPerms1PlugInfo = plugSnap4.Plugs["test-invalid-perms-1"]
	s.testInvalidPerms1Plug = interfaces.NewConnectedPlug(s.testInvalidPerms1PlugInfo, nil, nil)
}

func (s *PosixMQInterfaceSuite) checkSlotSeccompSnippet(c *C, spec *seccomp.Specification) {
	slotSnippet := spec.SnippetForTag("snap.producer.app")
	c.Check(slotSnippet, testutil.Contains, "mq_open")
	c.Check(slotSnippet, testutil.Contains, "mq_unlink")
	c.Check(slotSnippet, testutil.Contains, "mq_getsetattr")
	c.Check(slotSnippet, testutil.Contains, "mq_notify")
	c.Check(slotSnippet, testutil.Contains, "mq_timedreceive")
	c.Check(slotSnippet, testutil.Contains, "mq_timedsend")
}

func (s *PosixMQInterfaceSuite) TestReadWriteMQAppArmor(c *C) {
	spec := &apparmor.Specification{}
	err := spec.AddPermanentSlot(s.iface, s.testReadWriteSlotInfo)
	c.Assert(err, IsNil)
	err = spec.AddConnectedPlug(s.iface, s.testReadWritePlug, s.testReadWriteSlot)
	c.Assert(err, IsNil)
	c.Assert(spec.SecurityTags(), DeepEquals, []string{"snap.consumer.app", "snap.producer.app"})

	slotSnippet := spec.SnippetForTag("snap.producer.app")
	c.Check(slotSnippet, testutil.Contains, `mqueue (open read write create delete) "/test-rw",`)

	plugSnippet := spec.SnippetForTag("snap.consumer.app")
	c.Check(plugSnippet, testutil.Contains, `mqueue (read write open) "/test-rw",`)
}

func (s *PosixMQInterfaceSuite) TestReadWriteMQSeccomp(c *C) {
	spec := &seccomp.Specification{}
	err := spec.AddPermanentSlot(s.iface, s.testReadWriteSlotInfo)
	c.Assert(err, IsNil)
	err = spec.AddConnectedPlug(s.iface, s.testReadWritePlug, s.testReadWriteSlot)
	c.Assert(err, IsNil)
	c.Assert(spec.SecurityTags(), DeepEquals, []string{"snap.consumer.app", "snap.producer.app"})

	s.checkSlotSeccompSnippet(c, spec)
	plugSnippet := spec.SnippetForTag("snap.consumer.app")
	c.Check(plugSnippet, testutil.Contains, "mq_open")
	c.Check(plugSnippet, testutil.Contains, "mq_notify")
	c.Check(plugSnippet, testutil.Contains, "mq_timedreceive")
	c.Check(plugSnippet, testutil.Contains, "mq_timedsend")
	c.Check(plugSnippet, testutil.Contains, "mq_getsetattr")
	c.Check(plugSnippet, Not(testutil.Contains), "mq_unlink")
}

func (s *PosixMQInterfaceSuite) TestDefaultReadWriteMQAppArmor(c *C) {
	spec := &apparmor.Specification{}
	err := spec.AddPermanentSlot(s.iface, s.testDefaultPermsSlotInfo)
	c.Assert(err, IsNil)
	err = spec.AddConnectedPlug(s.iface, s.testDefaultPermsPlug, s.testDefaultPermsSlot)
	c.Assert(err, IsNil)
	c.Assert(spec.SecurityTags(), DeepEquals, []string{"snap.consumer.app", "snap.producer.app"})

	slotSnippet := spec.SnippetForTag("snap.producer.app")
	c.Check(slotSnippet, testutil.Contains, `mqueue (open read write create delete) "/test-default",`)

	plugSnippet := spec.SnippetForTag("snap.consumer.app")
	c.Check(plugSnippet, testutil.Contains, `mqueue (read write open) "/test-default",`)
}

func (s *PosixMQInterfaceSuite) TestDefaultReadWriteMQSeccomp(c *C) {
	spec := &seccomp.Specification{}
	err := spec.AddPermanentSlot(s.iface, s.testDefaultPermsSlotInfo)
	c.Assert(err, IsNil)
	err = spec.AddConnectedPlug(s.iface, s.testDefaultPermsPlug, s.testDefaultPermsSlot)
	c.Assert(err, IsNil)
	c.Assert(spec.SecurityTags(), DeepEquals, []string{"snap.consumer.app", "snap.producer.app"})

	s.checkSlotSeccompSnippet(c, spec)

	plugSnippet := spec.SnippetForTag("snap.consumer.app")
	c.Check(plugSnippet, testutil.Contains, "mq_open")
	c.Check(plugSnippet, testutil.Contains, "mq_notify")
	c.Check(plugSnippet, testutil.Contains, "mq_timedreceive")
	c.Check(plugSnippet, testutil.Contains, "mq_timedsend")
	c.Check(plugSnippet, testutil.Contains, "mq_getsetattr")
	c.Check(plugSnippet, Not(testutil.Contains), "mq_unlink")
}

func (s *PosixMQInterfaceSuite) TestReadOnlyMQAppArmor(c *C) {
	spec := &apparmor.Specification{}
	err := spec.AddPermanentSlot(s.iface, s.testReadOnlySlotInfo)
	c.Assert(err, IsNil)
	err = spec.AddConnectedPlug(s.iface, s.testReadOnlyPlug, s.testReadOnlySlot)
	c.Assert(err, IsNil)
	c.Assert(spec.SecurityTags(), DeepEquals, []string{"snap.consumer.app", "snap.producer.app"})

	slotSnippet := spec.SnippetForTag("snap.producer.app")
	c.Check(slotSnippet, testutil.Contains, `mqueue (open read write create delete) "/test-ro",`)

	plugSnippet := spec.SnippetForTag("snap.consumer.app")
	c.Check(plugSnippet, testutil.Contains, `mqueue (read open) "/test-ro",`)
}

func (s *PosixMQInterfaceSuite) TestReadOnlyMQSeccomp(c *C) {
	spec := &seccomp.Specification{}
	err := spec.AddPermanentSlot(s.iface, s.testReadOnlySlotInfo)
	c.Assert(err, IsNil)
	err = spec.AddConnectedPlug(s.iface, s.testReadOnlyPlug, s.testReadOnlySlot)
	c.Assert(err, IsNil)
	c.Assert(spec.SecurityTags(), DeepEquals, []string{"snap.consumer.app", "snap.producer.app"})

	s.checkSlotSeccompSnippet(c, spec)

	plugSnippet := spec.SnippetForTag("snap.consumer.app")
	c.Check(plugSnippet, testutil.Contains, "mq_open")
	c.Check(plugSnippet, testutil.Contains, "mq_notify")
	c.Check(plugSnippet, testutil.Contains, "mq_timedreceive")
	c.Check(plugSnippet, testutil.Contains, "mq_getsetattr")
	c.Check(plugSnippet, Not(testutil.Contains), "mq_timedsend")
	c.Check(plugSnippet, Not(testutil.Contains), "mq_unlink")
}

func (s *PosixMQInterfaceSuite) TestAllPermsMQAppArmor(c *C) {
	spec := &apparmor.Specification{}
	err := spec.AddPermanentSlot(s.iface, s.testAllPermsSlotInfo)
	c.Assert(err, IsNil)
	err = spec.AddConnectedPlug(s.iface, s.testAllPermsPlug, s.testAllPermsSlot)
	c.Assert(err, IsNil)
	c.Assert(spec.SecurityTags(), DeepEquals, []string{"snap.consumer.app", "snap.producer.app"})

	slotSnippet := spec.SnippetForTag("snap.producer.app")
	c.Check(slotSnippet, testutil.Contains, `mqueue (open read write create delete) "/test-all-perms",`)

	plugSnippet := spec.SnippetForTag("snap.consumer.app")
	c.Check(plugSnippet, testutil.Contains, `mqueue (create delete read write open) "/test-all-perms",`)
}

func (s *PosixMQInterfaceSuite) TestAllPermsMQSeccomp(c *C) {
	spec := &seccomp.Specification{}
	err := spec.AddPermanentSlot(s.iface, s.testAllPermsSlotInfo)
	c.Assert(err, IsNil)
	err = spec.AddConnectedPlug(s.iface, s.testAllPermsPlug, s.testAllPermsSlot)
	c.Assert(err, IsNil)
	c.Assert(spec.SecurityTags(), DeepEquals, []string{"snap.consumer.app", "snap.producer.app"})

	s.checkSlotSeccompSnippet(c, spec)

	plugSnippet := spec.SnippetForTag("snap.consumer.app")
	c.Check(plugSnippet, testutil.Contains, "mq_open")
	c.Check(plugSnippet, testutil.Contains, "mq_unlink")
	c.Check(plugSnippet, testutil.Contains, "mq_getsetattr")
	c.Check(plugSnippet, testutil.Contains, "mq_notify")
	c.Check(plugSnippet, testutil.Contains, "mq_timedreceive")
	c.Check(plugSnippet, testutil.Contains, "mq_timedsend")
}

func (s *PosixMQInterfaceSuite) TestPathValidationPosixMQ(c *C) {
	spec := &apparmor.Specification{}
	err := spec.AddPermanentSlot(s.iface, s.testInvalidPath1SlotInfo)
	c.Assert(err, NotNil)
}

func (s *PosixMQInterfaceSuite) TestPathValidationAppArmorRegex(c *C) {
	spec := &apparmor.Specification{}
	err := spec.AddPermanentSlot(s.iface, s.testInvalidPath2SlotInfo)
	c.Assert(err, NotNil)
}

func (s *PosixMQInterfaceSuite) TestPathStringValidation(c *C) {
	spec := &apparmor.Specification{}
	err := spec.AddPermanentSlot(s.iface, s.testInvalidPath3SlotInfo)
	c.Assert(err, NotNil)
}

func (s *PosixMQInterfaceSuite) testInvalidPerms1(c *C) {
	spec := &apparmor.Specification{}
	// The slot should function correctly here as it receives the full list
	// of built-in permissions, not what's listed in the configuration
	err := spec.AddPermanentSlot(s.iface, s.testInvalidPerms1SlotInfo)
	c.Assert(err, IsNil)
	// The plug should fail to connect as it receives the given list of
	// invalid permissions
	err = spec.AddConnectedPlug(s.iface, s.testInvalidPerms1Plug, s.testInvalidPerms1Slot)
	c.Assert(err, NotNil)
}

func (s *PosixMQInterfaceSuite) TestName(c *C) {
	c.Assert(s.iface.Name(), Equals, "posix-mq")
}

func (s *PosixMQInterfaceSuite) TestNoAppArmor(c *C) {
	// Ensure that the interface does not fail if AppArmor is unsupported
	restore := apparmor_sandbox.MockLevel(apparmor_sandbox.Unsupported)
	defer restore()
	c.Assert(interfaces.BeforePrepareSlot(s.iface, s.testReadWriteSlotInfo), IsNil)
}

func (s *PosixMQInterfaceSuite) TestFeatureDetection(c *C) {
	// Ensure that the interface fails if the mqueue feature is not present
	restore := apparmor_sandbox.MockFeatures([]string{}, nil, []string{}, nil)
	defer restore()
	c.Assert(interfaces.BeforePrepareSlot(s.iface, s.testReadWriteSlotInfo), NotNil)
}

func (s *PosixMQInterfaceSuite) TestSanitizeSlot(c *C) {
	// Ensure that the mqueue feature is detected
	restore := apparmor_sandbox.MockFeatures([]string{}, nil, []string{"mqueue"}, nil)
	defer restore()

	c.Assert(interfaces.BeforePrepareSlot(s.iface, s.testReadWriteSlotInfo), IsNil)
	c.Assert(interfaces.BeforePrepareSlot(s.iface, s.testDefaultPermsSlotInfo), IsNil)
	c.Assert(interfaces.BeforePrepareSlot(s.iface, s.testReadOnlySlotInfo), IsNil)
	c.Assert(interfaces.BeforePrepareSlot(s.iface, s.testAllPermsSlotInfo), IsNil)

	// These should return errors due to invalid configuration
	c.Assert(interfaces.BeforePrepareSlot(s.iface, s.testInvalidPath1SlotInfo), NotNil)
	c.Assert(interfaces.BeforePrepareSlot(s.iface, s.testInvalidPath2SlotInfo), NotNil)
	c.Assert(interfaces.BeforePrepareSlot(s.iface, s.testInvalidPath3SlotInfo), NotNil)
	c.Assert(interfaces.BeforePrepareSlot(s.iface, s.testInvalidPerms1SlotInfo), NotNil)
	c.Assert(interfaces.BeforePrepareSlot(s.iface, s.testInvalidPerms2SlotInfo), NotNil)
}

func (s *PosixMQInterfaceSuite) TestSanitizePlug(c *C) {
	// Ensure that the mqueue feature is detected
	restore := apparmor_sandbox.MockFeatures([]string{}, nil, []string{"mqueue"}, nil)
	defer restore()

	c.Assert(interfaces.BeforePreparePlug(s.iface, s.testReadWritePlugInfo), IsNil)
	c.Assert(interfaces.BeforePreparePlug(s.iface, s.testDefaultPermsPlugInfo), IsNil)
	c.Assert(interfaces.BeforePreparePlug(s.iface, s.testReadOnlyPlugInfo), IsNil)
	c.Assert(interfaces.BeforePreparePlug(s.iface, s.testAllPermsPlugInfo), IsNil)
	c.Assert(interfaces.BeforePreparePlug(s.iface, s.testInvalidPerms1PlugInfo), IsNil)
}

func (s *PosixMQInterfaceSuite) TestInterfaces(c *C) {
	c.Check(builtin.Interfaces(), testutil.DeepContains, s.iface)
}

func (s *PosixMQInterfaceSuite) TestAutoConnect(c *C) {
	c.Assert(s.iface.AutoConnect(s.testReadWritePlugInfo, s.testReadWriteSlotInfo), Equals, true)
}

func (s *PosixMQInterfaceSuite) TestStaticInfo(c *C) {
	si := interfaces.StaticInfoOf(s.iface)
	c.Check(si.ImplicitOnCore, Equals, false)
	c.Check(si.ImplicitOnClassic, Equals, false)
	c.Check(si.Summary, Equals, `allows access to POSIX message queues`)
}
