"""
Copyright (c) 2016-present, Facebook, Inc.
All rights reserved.

This source code is licensed under the BSD-style license found in the
LICENSE file in the root directory of this source tree. An additional grant
of patent rights can be found in the PATENTS file in the same directory.
"""

import unittest

import s1ap_types
import s1ap_wrapper
from integ_tests.s1aptests.ovs.rest_api import get_datapath, get_flows
from magma.pipelined.imsi import decode_imsi


class TestAttachDetachWithOVS(unittest.TestCase):

    SPGW_TABLE = 0
    GTP_PORT = 32768

    def setUp(self):
        self._s1ap_wrapper = s1ap_wrapper.TestWrapper()

    def tearDown(self):
        self._s1ap_wrapper.cleanup()

    def check_imsi_metadata(self, flow, ue_req):
        """ Checks that IMSI set in the flow metadata matches the one sent """
        sent_imsi = 'IMSI' + ''.join([str(i) for i in ue_req.imsi])
        imsi_action = next((a for a in flow["instructions"][0]["actions"]
                            if a["field"] == "metadata"), None)
        self.assertIsNotNone(imsi_action)
        imsi64 = imsi_action["value"]
        # Convert between compacted uint IMSI and string
        received_imsi = decode_imsi(imsi64)
        self.assertEqual(sent_imsi, received_imsi,
                         "IMSI set in metadata field does not match sent IMSI")

    def test_attach_detach_with_ovs(self):
        """
        Basic sanity check of UE downlink/uplink flows during attach and
        detach procedures.
        """
        datapath = get_datapath()

        print("Checking for default table 0 flows")
        flows = get_flows(datapath, {"table_id": self.SPGW_TABLE,
                                          "priority": 0})
        self.assertEqual(len(flows), 1,
                         "There should only be 1 default table 0 flow")

        self._s1ap_wrapper.configUEDevice(1)
        req = self._s1ap_wrapper.ue_req

        print("Running End to End attach for UE id ", req.ue_id)
        self._s1ap_wrapper._s1_util.attach(
            req.ue_id, s1ap_types.tfwCmd.UE_END_TO_END_ATTACH_REQUEST,
            s1ap_types.tfwCmd.UE_ATTACH_ACCEPT_IND,
            s1ap_types.ueAttachAccept_t)

        self._s1ap_wrapper._s1_util.receive_emm_info()

        # UPLINK
        print("Checking for uplink flow")
        uplink_flows = get_flows(datapath,
                                      {"table_id": self.SPGW_TABLE,
                                       "match": {"in_port": self.GTP_PORT}})
        self.assertEqual(len(uplink_flows), 1, "Uplink flow missing for UE")
        self.assertIsNotNone(uplink_flows[0]["match"]["tunnel_id"],
                             "Uplink flow missing tunnel id match")
        self.check_imsi_metadata(uplink_flows[0], req)

        # DOWNLINK
        print("Checking for downlink flow")
        ue_ip = str(self._s1ap_wrapper._s1_util.get_ip(req.ue_id))
        # Ryu can't match on ipv4_dst, so match on uplink in port
        downlink_flows = get_flows(datapath,
                                        {"table_id": self.SPGW_TABLE,
                                         "match": {"nw_dst": ue_ip,
                                                   "eth_type": 2048}})
        self.assertEqual(len(downlink_flows), 1, "Downlink flow missing for UE")
        self.assertEqual(downlink_flows[0]["match"]["ipv4_dst"], ue_ip,
                         "UE IP match missing from downlink flow")

        actions = downlink_flows[0]["instructions"][0]["actions"]
        has_tunnel_action = any(action for action in actions
                                if action["field"] == "tunnel_id" and
                                action["type"] == "SET_FIELD")
        self.assertTrue(has_tunnel_action,
                        "Downlink flow missing set tunnel action")
        self.check_imsi_metadata(downlink_flows[0], req)

        print("Running UE detach for UE id ", req.ue_id)
        # Now detach the UE
        self._s1ap_wrapper.s1_util.detach(
            req.ue_id, s1ap_types.ueDetachType_t.UE_NORMAL_DETACH.value, True)

        print("Checking that uplink/downlink flows were deleted")
        flows = get_flows(datapath, {"table_id": self.SPGW_TABLE,
                                          "priority": 0})
        self.assertEqual(len(flows), 1,
                         "There should only be 1 default table 0 flow")


if __name__ == "__main__":
    unittest.main()
