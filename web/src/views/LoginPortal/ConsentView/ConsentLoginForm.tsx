import React, { MutableRefObject, useCallback, useEffect, useMemo, useRef, useState } from "react";

import { Alert, AlertTitle, Button, FormControl, Grid } from "@mui/material";
import TextField from "@mui/material/TextField";
import { BroadcastChannel } from "broadcast-channel";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router-dom";

import { RedirectionURL, RequestMethod } from "@constants/SearchParams";
import { useNotifications } from "@hooks/NotificationsContext";
import { useQueryParam } from "@hooks/QueryParam";
import { useWorkflow } from "@hooks/Workflow";
import LoginLayout from "@layouts/LoginLayout";
import { IsCapsLockModified } from "@services/CapsLock";
import { postFirstFactorReauthenticate } from "@services/FirstFactor";

export interface Props {
    rememberMe: boolean;

    resetPassword: boolean;
    resetPasswordCustomURL: string;

    onAuthenticationStart: () => void;
    onAuthenticationFailure: () => void;
    onAuthenticationSuccess: (redirectURL: string | undefined) => void;
    onChannelStateChange: () => void;
}

const ConsentLoginForm = function (props: Props) {
    const { t: translate } = useTranslation();

    const navigate = useNavigate();
    const redirectionURL = useQueryParam(RedirectionURL);
    const requestMethod = useQueryParam(RequestMethod);
    const [workflow, workflowID] = useWorkflow();
    const { createErrorNotification } = useNotifications();

    const loginChannel = useMemo(() => new BroadcastChannel<boolean>("login"), []);

    const [disabled, setDisabled] = useState(false);
    const [password, setPassword] = useState("");
    const [passwordCapsLock, setPasswordCapsLock] = useState(false);
    const [passwordCapsLockPartial, setPasswordCapsLockPartial] = useState(false);
    const [passwordError, setPasswordError] = useState(false);

    const passwordRef = useRef() as MutableRefObject<HTMLInputElement>;

    useEffect(() => {
        const timeout = setTimeout(() => passwordRef.current.focus(), 10);
        return () => clearTimeout(timeout);
    }, [passwordRef]);

    useEffect(() => {
        loginChannel.addEventListener("message", (authenticated) => {
            if (authenticated) {
                props.onChannelStateChange();
            }
        });
    }, [loginChannel, redirectionURL, props]);

    const handleSignIn = useCallback(async () => {
        if (password === "") {
            setPasswordError(true);

            return;
        }

        props.onAuthenticationStart();
        try {
            const res = await postFirstFactorReauthenticate(
                password,
                redirectionURL,
                requestMethod,
                workflow,
                workflowID,
            );
            await loginChannel.postMessage(true);
            props.onAuthenticationSuccess(res ? res.redirect : undefined);
        } catch (err) {
            console.error(err);
            createErrorNotification(translate("Incorrect username or password"));
            props.onAuthenticationFailure();
            setPassword("");
            passwordRef.current.focus();
        }
    }, [
        createErrorNotification,
        loginChannel,
        password,
        props,
        redirectionURL,
        requestMethod,
        translate,
        workflow,
        workflowID,
    ]);

    const handlePasswordKeyDown = useCallback(
        (event: React.KeyboardEvent<HTMLDivElement>) => {
            if (event.key === "Enter") {
                if (!password.length) {
                    passwordRef.current.focus();
                }
                handleSignIn().catch(console.error);
                event.preventDefault();
            }
        },
        [handleSignIn, password.length],
    );

    const handlePasswordKeyUp = useCallback(
        (event: React.KeyboardEvent<HTMLDivElement>) => {
            if (password.length <= 1) {
                setPasswordCapsLock(false);
                setPasswordCapsLockPartial(false);

                if (password.length === 0) {
                    return;
                }
            }

            const modified = IsCapsLockModified(event);

            if (modified === null) return;

            if (modified) {
                setPasswordCapsLock(true);
            } else {
                setPasswordCapsLockPartial(true);
            }
        },
        [password.length],
    );

    return (
        <LoginLayout id="first-factor-reauthenticate-stage" title={translate("Confirm access")}>
            <FormControl id={"form-reauthenticate"}>
                <Grid container spacing={2}>
                    <Grid item xs={12}>
                        <TextField
                            inputRef={passwordRef}
                            id="password-textfield"
                            label={translate("Password")}
                            variant="outlined"
                            required
                            fullWidth
                            disabled={disabled}
                            value={password}
                            error={passwordError}
                            onChange={(v) => setPassword(v.target.value)}
                            onFocus={() => setPasswordError(false)}
                            type="password"
                            autoComplete="current-password"
                            onKeyDown={handlePasswordKeyDown}
                            onKeyUp={handlePasswordKeyUp}
                        />
                    </Grid>
                    {passwordCapsLock ? (
                        <Grid item xs={12} marginX={2}>
                            <Alert severity={"warning"}>
                                <AlertTitle>{translate("Warning")}</AlertTitle>
                                {passwordCapsLockPartial
                                    ? translate("The password was partially entered with Caps Lock")
                                    : translate("The password was entered with Caps Lock")}
                            </Alert>
                        </Grid>
                    ) : null}
                    <Grid item xs={12}>
                        <Button
                            id="sign-in-button"
                            variant="contained"
                            color="primary"
                            fullWidth
                            disabled={disabled}
                            onClick={handleSignIn}
                        >
                            {translate("Confirm")}
                        </Button>
                    </Grid>
                </Grid>
            </FormControl>
        </LoginLayout>
    );
};

export default ConsentLoginForm;
