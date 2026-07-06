import React, { useState } from "react";
import { useEffect } from "react";
import { useNavigate, useParams } from "react-router-dom";

import axios from "../../axios";

import Dashboard from "../../Dashboard";
import {
  Button,
  Grid,
  FormControl,
  InputLabel,
  Select,
  MenuItem,
  LinearProgress,
  Typography,
  Alert,
} from "@mui/material";
import { SubscriberFormProvider, useSubscriptionForm } from "../../hooks/subscription-form";
import SubscriberFormBasic from "./SubscriberFormBasic";
import SubscriberFormUeAmbr from "./SubscriberFormUeAmbr";
import SubscriberFormSessions from "./SubscriberFormSessions";
import { FlowsMapperImpl as SubscriptionFlowsMapperImpl, SubscriptionMapperImpl } from "../../lib/dtos/subscription";
import { FlowsMapperImpl as ProfileFlowsMapperImpl, ProfileMapperImpl } from "../../lib/dtos/profile";
import { validateSubscription } from "../../lib/validator/subscriptionValidator";

function FormHOC(Component: React.ComponentType<any>) {
  return function (props: any) {
    return (
      <SubscriberFormProvider>
        <Component {...props} />
      </SubscriberFormProvider>
    );
  };
}

export default FormHOC(SubscriberCreate);

function SubscriberCreate() {
  const { id, plmn } = useParams<{
    id: string;
    plmn: string;
  }>();

  const isNewSubscriber = id === undefined && plmn === undefined;
  const navigation = useNavigate();
  const [loading, setLoading] = useState(false);
  const [profiles, setProfiles] = useState<string[]>([]);
  const [selectedProfile, setSelectedProfile] = useState("");

  // Track server-side bulk creation progress.
  const [createProgress, setCreateProgress] = useState<{
    current: number;
    total: number;
  } | null>(null);

  // Show create errors without leaving the form.
  const [createErrors, setCreateErrors] = useState<string[]>([]);

  const { handleSubmit, getValues, reset } = useSubscriptionForm();

  useEffect(() => {
    axios
      .get("/api/profile")
      .then((res) => {
        setProfiles(res.data);
      })
      .catch((e) => {
        console.log(e.message);
      });
  }, []);

  if (!isNewSubscriber) {
    useEffect(() => {
      setLoading(true);

      axios
        .get("/api/subscriber/" + id + "/" + plmn)
        .then((res) => {
          const subscriberMapper = new SubscriptionMapperImpl(new SubscriptionFlowsMapperImpl());
          const subscription = subscriberMapper.mapFromSubscription(res.data);
          reset(subscription);
        })
        .finally(() => {
          setLoading(false);
        });
    }, [id]);
  }

  if (loading) {
    return <div>Loading...</div>;
  }

  const onCreate = async () => {
    console.log("trace: onCreate");

    const data = getValues();

    if (data.SnssaiConfigurations.length === 0) {
      alert("Please add at least one S-NSSAI");
      return;
    }

    const subscriberMapper = new SubscriptionMapperImpl(new SubscriptionFlowsMapperImpl());
    const subscription = subscriberMapper.mapFromDto(data);

    const validation = validateSubscription(subscription);
    if (!validation.isValid) {
      alert(validation.error);
      return;
    }

    const total = subscription.userNumber!;
    setCreateErrors([]);
    setCreateProgress({ current: 0, total });

    try {
      const createURL =
        "/api/subscriber/" +
        subscription.ueId +
        "/" +
        subscription.plmnID +
        "/" +
        total;
      const res = await axios.post(
        createURL,
        subscription
      );
      setCreateProgress({ current: res.data?.created ?? total, total });
      navigation("/subscriber");
    } catch (err: any) {
      if (err.response) {
        const msg = `HTTP ${err.response.status}${
          err.response.data?.cause ? ": " + err.response.data.cause : ""
        }`;
        setCreateErrors([msg]);
      } else {
        setCreateErrors([err.message]);
      }
    } finally {
      setCreateProgress(null);
    }
  };

  const onUpdate = () => {
    console.log("trace: onUpdate");

    const data = getValues();
    const subscriberMapper = new SubscriptionMapperImpl(new SubscriptionFlowsMapperImpl());
    const subscription = subscriberMapper.mapFromDto(data);

    const validation = validateSubscription(subscription);
    if (!validation.isValid) {
      alert(validation.error);
      return;
    }

    axios
      .put(
        "/api/subscriber/" + subscription.ueId + "/" + subscription.plmnID,
        subscription
      )
      .then(() => {
        navigation(
          "/subscriber/" + subscription.ueId + "/" + subscription.plmnID
        );
      })
      .catch((err) => {
        if (err.response) {
          const msg = "Status: " + err.response.status;
          if (err.response.data.cause) {
            alert(msg + ", cause: " + err.response.data.cause);
          } else if (err.response.data) {
            alert(msg + ", data:" + err.response.data);
          } else {
            alert(msg);
          }
        } else {
          alert(err.message);
        }
      });
  };

  const formSubmitFn = isNewSubscriber ? onCreate : onUpdate;
  const formSubmitText = isNewSubscriber ? "CREATE" : "UPDATE";
  const isCreating = createProgress !== null;
  const progressPercent = isCreating
    ? Math.round((createProgress.current / createProgress.total) * 100)
    : 0;

  const handleProfileChange = (event: any) => {
    const profileName = event.target.value;
    setSelectedProfile(profileName);

    if (profileName) {
      setLoading(true);
      axios
        .get("/api/profile/" + profileName)
        .then((res) => {
          const profileMapper = new ProfileMapperImpl(new ProfileFlowsMapperImpl());
          const profile = profileMapper.mapFromProfile(res.data);

          const currentValues = getValues();
          const basicInfo = {
            userNumber: currentValues.userNumber,
            ueId: currentValues.ueId,
            plmnID: currentValues.plmnID,
            gpsi: currentValues.gpsi,
            auth: {
              authenticationManagementField:
                currentValues.auth?.authenticationManagementField,
              authenticationMethod: currentValues.auth?.authenticationMethod,
              operatorCodeType: currentValues.auth?.operatorCodeType,
              operatorCode: currentValues.auth?.operatorCode,
              sequenceNumber: currentValues.auth?.sequenceNumber,
              permanentKey: currentValues.auth?.permanentKey,
            },
          };

          reset({
            ...basicInfo,
            ...profile,
          });
        })
        .catch((e) => {
          console.log(e.message);
        })
        .finally(() => {
          setLoading(false);
        });
    }
  };

  return (
    <Dashboard title="Subscription" refreshAction={() => {}}>
      <form
        onSubmit={handleSubmit(formSubmitFn, (err) => {
          console.log("form error: ", err);
        })}
      >
        {profiles.length > 0 && (
          <Grid item xs={12} sx={{ mb: 3 }}>
            <FormControl fullWidth>
              <InputLabel id="profile-select-label">Select Profile</InputLabel>
              <Select
                labelId="profile-select-label"
                id="profile-select"
                value={selectedProfile}
                label="Select Profile"
                onChange={handleProfileChange}
              >
                {profiles.map((profile) => (
                  <MenuItem key={profile} value={profile}>
                    {profile}
                  </MenuItem>
                ))}
              </Select>
            </FormControl>
          </Grid>
        )}

        <SubscriberFormBasic />

        <h3>Subscribed UE AMBR</h3>
        <SubscriberFormUeAmbr />

        <SubscriberFormSessions />

        <br />

        {/* Progress bar shown while the server creates the requested range */}
        {isCreating && (
          <Grid item xs={12} sx={{ mb: 2 }}>
            <Typography variant="body2" sx={{ mb: 0.5 }}>
              Creating subscribers: {createProgress.current} / {createProgress.total}
            </Typography>
            <LinearProgress variant="determinate" value={progressPercent} />
          </Grid>
        )}

        {/* Error summary shown after create failure */}
        {createErrors.length > 0 && (
          <Grid item xs={12} sx={{ mb: 2 }}>
            <Alert severity="error">
              <strong>
                {createErrors.length} subscriber(s) failed to create:
              </strong>
              <ul style={{ margin: "4px 0 0 0", paddingLeft: "1.2em" }}>
                {createErrors.slice(0, 20).map((e, i) => (
                  <li key={i}>{e}</li>
                ))}
                {createErrors.length > 20 && (
                  <li>...and {createErrors.length - 20} more</li>
                )}
              </ul>
            </Alert>
          </Grid>
        )}

        <Grid item xs={12}>
          <Button
            color="primary"
            variant="contained"
            type="submit"
            disabled={isCreating}
            sx={{ m: 1 }}
          >
            {isCreating
              ? `Creating… ${progressPercent}%`
              : formSubmitText}
          </Button>
        </Grid>
      </form>
    </Dashboard>
  );
}
