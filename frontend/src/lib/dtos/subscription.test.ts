import { test, describe } from "vitest";
import { Subscription } from "../../api";
import {
  FlowsMapperImpl,
  SubscriptionMapperImpl,
  SubscriptionDTO,
  defaultSubscriptionDTO,
  subscriptionDTOSchema,
} from "./subscription";
import assert from "node:assert";

const defaultSubscription = (): Subscription => ({
  userNumber: 1,
  plmnID: "20893",
  ueId: "imsi-208930000000001",
  AuthenticationSubscription: {
    authenticationMethod: "5G_AKA",
    sequenceNumber: "000000000023",
    authenticationManagementField: "8000",
    permanentKey: {
      permanentKeyValue: "8baf473f2f8fd09487cccbd7097c6862",
      encryptionKey: 0,
      encryptionAlgorithm: 0,
    },
    milenage: {
      op: {
        opValue: "",
        encryptionKey: 0,
        encryptionAlgorithm: 0,
      },
    },
    opc: {
      opcValue: "8e27b6af0e692e750f32667a3b14605d",
      encryptionKey: 0,
      encryptionAlgorithm: 0,
    },
  },
  AccessAndMobilitySubscriptionData: {
    gpsis: ["msisdn-"],
    subscribedUeAmbr: {
      uplink: "1 Gbps",
      downlink: "2 Gbps",
    },
    nssai: {
      defaultSingleNssais: [
        {
          sst: 1,
          sd: "010203",
        },
      ],
      singleNssais: [
        {
          sst: 1,
          sd: "112233",
        },
      ],
    },
  },
  SessionManagementSubscriptionData: [
    {
      singleNssai: {
        sst: 1,
        sd: "010203",
      },
      dnnConfigurations: {
        internet: {
          pduSessionTypes: {
            defaultSessionType: "IPV4",
            allowedSessionTypes: ["IPV4"],
          },
          sscModes: {
            defaultSscMode: "SSC_MODE_1",
            allowedSscModes: ["SSC_MODE_2", "SSC_MODE_3"],
          },
          "5gQosProfile": {
            "5qi": 9,
            arp: {
              priorityLevel: 8,
              preemptCap: "",
              preemptVuln: "",
            },
            priorityLevel: 8,
          },
          sessionAmbr: {
            uplink: "1000 Mbps",
            downlink: "1000 Mbps",
          },
          staticIpAddress: [],
        },
      },
    },
    {
      singleNssai: {
        sst: 1,
        sd: "112233",
      },
      dnnConfigurations: {
        internet: {
          pduSessionTypes: {
            defaultSessionType: "IPV4",
            allowedSessionTypes: ["IPV4"],
          },
          sscModes: {
            defaultSscMode: "SSC_MODE_1",
            allowedSscModes: ["SSC_MODE_2", "SSC_MODE_3"],
          },
          "5gQosProfile": {
            "5qi": 8,
            arp: {
              priorityLevel: 8,
              preemptCap: "",
              preemptVuln: "",
            },
            priorityLevel: 8,
          },
          sessionAmbr: {
            uplink: "1000 Mbps",
            downlink: "1000 Mbps",
          },
          staticIpAddress: [],
        },
      },
    },
  ],
  SmfSelectionSubscriptionData: {
    subscribedSnssaiInfos: {
      "01010203": {
        dnnInfos: [
          {
            dnn: "internet",
          },
        ],
      },
      "01112233": {
        dnnInfos: [
          {
            dnn: "internet",
          },
        ],
      },
    },
  },
  AmPolicyData: {
    subscCats: ["free5gc"],
  },
  SmPolicyData: {
    smPolicySnssaiData: {
      "01010203": {
        snssai: {
          sst: 1,
          sd: "010203",
        },
        smPolicyDnnData: {
          internet: {
            dnn: "internet",
          },
        },
      },
      "01112233": {
        snssai: {
          sst: 1,
          sd: "112233",
        },
        smPolicyDnnData: {
          internet: {
            dnn: "internet",
          },
        },
      },
    },
  },
  FlowRules: [
    {
      filter: "1.1.1.1/32",
      precedence: 128,
      snssai: "01010203",
      dnn: "internet",
      qosRef: 1,
    },
    {
      filter: "1.1.1.1/32",
      precedence: 127,
      snssai: "01112233",
      dnn: "internet",
      qosRef: 2,
    },
  ],
  QosFlows: [
    {
      snssai: "01010203",
      dnn: "internet",
      qosRef: 1,
      "5qi": 8,
      mbrUL: "208 Mbps",
      mbrDL: "208 Mbps",
      gbrUL: "108 Mbps",
      gbrDL: "108 Mbps",
    },
    {
      snssai: "01112233",
      dnn: "internet",
      qosRef: 2,
      "5qi": 7,
      mbrUL: "407 Mbps",
      mbrDL: "407 Mbps",
      gbrUL: "207 Mbps",
      gbrDL: "207 Mbps",
    },
  ],
  ChargingDatas: [
    {
      snssai: "01010203",
      dnn: "",
      filter: "",
      chargingMethod: "Offline",
      quota: "100000",
      unitCost: "1",
    },
    {
      snssai: "01010203",
      dnn: "internet",
      qosRef: 1,
      filter: "1.1.1.1/32",
      chargingMethod: "Offline",
      quota: "100000",
      unitCost: "1",
    },
    {
      snssai: "01112233",
      dnn: "",
      filter: "",
      chargingMethod: "Online",
      quota: "100000",
      unitCost: "1",
    },
    {
      snssai: "01112233",
      dnn: "internet",
      qosRef: 2,
      filter: "1.1.1.1/32",
      chargingMethod: "Online",
      quota: "5000",
      unitCost: "1",
    },
  ],
});

describe("SubscriptionDTO", ({ extend }) => {
  describe("default subscription", () => {
    test("build", ({ expect }) => {
      const subscriptionBuilder = new SubscriptionMapperImpl(new FlowsMapperImpl());
      const subscription = subscriptionBuilder.mapFromDto(defaultSubscriptionDTO());
      assert.deepEqual(JSON.parse(JSON.stringify(subscription)), defaultSubscription());
    });

    test("parse", ({ expect }) => {
      const subscriptionBuilder = new SubscriptionMapperImpl(new FlowsMapperImpl());
      const subscription = subscriptionBuilder.mapFromSubscription(defaultSubscription());
      assert.deepEqual(JSON.parse(JSON.stringify(subscription)), defaultSubscriptionDTO());
    });
  });

  describe("subscription with up security", () => {
    const subscriptionWithUpSecurity: Subscription = defaultSubscription();
    subscriptionWithUpSecurity.SessionManagementSubscriptionData[0].dnnConfigurations![
      "internet"
    ].upSecurity = {
      upIntegr: "1 Gbps",
      upConfid: "2 Gbps",
    };

    const subscriptionWithUpSecurityDTO: SubscriptionDTO = defaultSubscriptionDTO();
    subscriptionWithUpSecurityDTO.SnssaiConfigurations[0].dnnConfigurations["internet"].upSecurity =
      {
        upIntegr: "1 Gbps",
        upConfid: "2 Gbps",
      };

    test("build", ({ expect }) => {
      const subscriptionBuilder = new SubscriptionMapperImpl(new FlowsMapperImpl());
      const subscription = subscriptionBuilder.mapFromDto(subscriptionWithUpSecurityDTO);
      expect(subscription).toEqual(subscriptionWithUpSecurity);
    });

    test("parse", () => {
      const subscriptionBuilder = new SubscriptionMapperImpl(new FlowsMapperImpl());
      const subscription = subscriptionBuilder.mapFromSubscription(subscriptionWithUpSecurity);
      assert.deepEqual(
        JSON.parse(JSON.stringify(subscription)),
        JSON.parse(JSON.stringify(subscriptionWithUpSecurityDTO)),
      );
    });
  });

  describe("OPc subscription", () => {
    const subscriptionWithUpSecurity: Subscription = defaultSubscription();
    subscriptionWithUpSecurity.AuthenticationSubscription.milenage!.op!.opValue = "";
    subscriptionWithUpSecurity.AuthenticationSubscription!.opc!.opcValue =
      "8e27b6af0e692e750f32667a3b14605d";

    const subscriptionWithUpSecurityDTO: SubscriptionDTO = defaultSubscriptionDTO();
    subscriptionWithUpSecurityDTO.auth.operatorCodeType = "OPc";
    subscriptionWithUpSecurityDTO.auth.operatorCode = "8e27b6af0e692e750f32667a3b14605d";

    test("build", () => {
      const subscriptionBuilder = new SubscriptionMapperImpl(new FlowsMapperImpl());
      const subscription = subscriptionBuilder.mapFromDto(subscriptionWithUpSecurityDTO);
      assert.deepEqual(
        JSON.parse(JSON.stringify(subscription)),
        JSON.parse(JSON.stringify(subscriptionWithUpSecurity)),
      );
    });

    test("parse", () => {
      const subscriptionBuilder = new SubscriptionMapperImpl(new FlowsMapperImpl());
      const subscription = subscriptionBuilder.mapFromSubscription(subscriptionWithUpSecurity);
      assert.deepEqual(
        JSON.parse(JSON.stringify(subscription)),
        JSON.parse(JSON.stringify(subscriptionWithUpSecurityDTO)),
      );
    });
  });

  describe("OP subscription", () => {
    const subscriptionWithUpSecurity: Subscription = defaultSubscription();
    subscriptionWithUpSecurity.AuthenticationSubscription.milenage!.op!.opValue =
      "8baf473f2f8fd09487cccbd7097c6862";
    subscriptionWithUpSecurity.AuthenticationSubscription!.opc!.opcValue = "";

    const subscriptionWithUpSecurityDTO: SubscriptionDTO = defaultSubscriptionDTO();
    subscriptionWithUpSecurityDTO.auth.operatorCodeType = "OP";
    subscriptionWithUpSecurityDTO.auth.operatorCode = "8baf473f2f8fd09487cccbd7097c6862";

    test("build", () => {
      const subscriptionBuilder = new SubscriptionMapperImpl(new FlowsMapperImpl());
      const subscription = subscriptionBuilder.mapFromDto(subscriptionWithUpSecurityDTO);
      assert.deepEqual(
        JSON.parse(JSON.stringify(subscription)),
        JSON.parse(JSON.stringify(subscriptionWithUpSecurity)),
      );
    });

    test("parse", () => {
      const subscriptionBuilder = new SubscriptionMapperImpl(new FlowsMapperImpl());
      const subscription = subscriptionBuilder.mapFromSubscription(subscriptionWithUpSecurity);
      assert.deepEqual(
        JSON.parse(JSON.stringify(subscription)),
        JSON.parse(JSON.stringify(subscriptionWithUpSecurityDTO)),
      );
    });
  });
  describe("subscription with DualSteer group ID", () => {
    const dualSteerSubscription: Subscription = defaultSubscription();
    dualSteerSubscription.SessionManagementSubscriptionData[0].internalGroupIds = [
      "0000abcd-208-93-01",
      "00004453-208-93-0a0b",
    ];
    const dualSteerDTO: SubscriptionDTO = defaultSubscriptionDTO();
    dualSteerDTO.SnssaiConfigurations[0].dsGroupId = "00004453-208-93-0a0b";
    dualSteerDTO.SnssaiConfigurations[0].otherInternalGroupIds = ["0000abcd-208-93-01"];

    test("build", () => {
      const subscriptionBuilder = new SubscriptionMapperImpl(new FlowsMapperImpl());
      const subscription = subscriptionBuilder.mapFromDto(dualSteerDTO);
      assert.deepEqual(JSON.parse(JSON.stringify(subscription)), dualSteerSubscription);
    });
    test("parse", () => {
      const subscriptionBuilder = new SubscriptionMapperImpl(new FlowsMapperImpl());
      const dto = subscriptionBuilder.mapFromSubscription(dualSteerSubscription);
      assert.deepEqual(JSON.parse(JSON.stringify(dto)), dualSteerDTO);
    });
    test("reopening and saving keeps internalGroupIds", () => {
      const subscriptionBuilder = new SubscriptionMapperImpl(new FlowsMapperImpl());
      const reopened = subscriptionBuilder.mapFromSubscription(dualSteerSubscription);
      const saved = new SubscriptionMapperImpl(new FlowsMapperImpl()).mapFromDto(reopened);
      assert.deepEqual(saved.SessionManagementSubscriptionData[0].internalGroupIds, [
        "0000abcd-208-93-01",
        "00004453-208-93-0a0b",
      ]);
      assert.equal(saved.SessionManagementSubscriptionData[1].internalGroupIds, undefined);
    });
    test("build S-NSSAI without group ID fields, as after applying a profile", () => {
      const dto = defaultSubscriptionDTO();
      delete dto.SnssaiConfigurations[0].dsGroupId;
      delete dto.SnssaiConfigurations[0].otherInternalGroupIds;
      assert.equal(subscriptionDTOSchema.safeParse(dto).success, true);
      const subscription = new SubscriptionMapperImpl(new FlowsMapperImpl()).mapFromDto(dto);
      assert.deepEqual(JSON.parse(JSON.stringify(subscription)), defaultSubscription());
    });
  });

  describe("subscription read back without staticIpAddress", () => {
    // The backend omits an empty staticIpAddress, so a saved subscription comes back without it.
    const readBack: Subscription = defaultSubscription();
    delete readBack.SessionManagementSubscriptionData[0].dnnConfigurations!["internet"]
      .staticIpAddress;

    test("parse", () => {
      const dto = new SubscriptionMapperImpl(new FlowsMapperImpl()).mapFromSubscription(readBack);
      assert.equal(
        dto.SnssaiConfigurations[0].dnnConfigurations["internet"].enableStaticIpv4Address,
        false,
      );
    });
  });

  describe("DualSteer group ID validation", () => {
    const withGroupId = (dsGroupId: string) => {
      const dto = defaultSubscriptionDTO();
      dto.SnssaiConfigurations[0].dsGroupId = dsGroupId;
      return dto;
    };

    test.each(["", "00004453-208-93-01", "00004453-466-001-0102030405060708090A"])(
      "accepts %j",
      (dsGroupId) => {
        assert.equal(subscriptionDTOSchema.safeParse(withGroupId(dsGroupId)).success, true);
      },
    );
    test.each([
      "00004453-208-93",
      "00004453-20-93-01",
      "00004453-208-93-0",
      "00004453-208-93-0102030405060708090a0b",
      "0000abcd-208-93-01",
      "dualsteer-1",
    ])("rejects %j", (dsGroupId) => {
      assert.equal(subscriptionDTOSchema.safeParse(withGroupId(dsGroupId)).success, false);
    });
  });
});
