import React, { CSSProperties, MutableRefObject, useEffect, useRef, useState } from "react";

import { Grid, Button, FormControlLabel, Checkbox, Link, Theme, useTheme } from "@mui/material";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router-dom";

import FixedTextField from "@components/FixedTextField";
import { ResetPasswordStep1Route } from "@constants/Routes";
import { useNotifications } from "@hooks/NotificationsContext";
import { useRedirectionURL } from "@hooks/RedirectionURL";
import { useRequestMethod } from "@hooks/RequestMethod";
import LoginLayout from "@layouts/LoginLayout";
import { postFirstFactor } from "@services/FirstFactor";

export interface Props {
    disabled: boolean;
    rememberMe: boolean;
    resetPassword: boolean;

    onAuthenticationStart: () => void;
    onAuthenticationFailure: () => void;
    onAuthenticationSuccess: (redirectURL: string | undefined) => void;
}

const FirstFactorForm = function (props: Props) {
    const theme = useTheme();
    const style = useStyles(theme);

    const navigate = useNavigate();
    const redirectionURL = useRedirectionURL();
    const requestMethod = useRequestMethod();

    const [rememberMe, setRememberMe] = useState(false);
    const [username, setUsername] = useState("");
    const [usernameError, setUsernameError] = useState(false);
    const [password, setPassword] = useState("");
    const [passwordError, setPasswordError] = useState(false);
    const { createErrorNotification } = useNotifications();
    // TODO (PR: #806, Issue: #511) potentially refactor
    const usernameRef = useRef() as MutableRefObject<HTMLInputElement>;
    const passwordRef = useRef() as MutableRefObject<HTMLInputElement>;
    const { t: translate } = useTranslation("Portal");
    useEffect(() => {
        const timeout = setTimeout(() => usernameRef.current.focus(), 10);
        return () => clearTimeout(timeout);
    }, [usernameRef]);

    const disabled = props.disabled;

    const handleRememberMeChange = () => {
        setRememberMe(!rememberMe);
    };

    const handleSignIn = async () => {
        if (username === "" || password === "") {
            if (username === "") {
                setUsernameError(true);
            }

            if (password === "") {
                setPasswordError(true);
            }
            return;
        }

        props.onAuthenticationStart();
        try {
            const res = await postFirstFactor(username, password, rememberMe, redirectionURL, requestMethod);
            props.onAuthenticationSuccess(res ? res.redirect : undefined);
        } catch (err) {
            console.error(err);
            createErrorNotification(translate("Incorrect username or password"));
            props.onAuthenticationFailure();
            setPassword("");
            passwordRef.current.focus();
        }
    };

    const handleResetPasswordClick = () => {
        navigate(ResetPasswordStep1Route);
    };

    // @ts-ignore
    return (
        <LoginLayout id="first-factor-stage" title={translate("Sign in")} showBrand>
            <Grid container spacing={2}>
                <Grid item xs={12}>
                    <FixedTextField
                        // TODO (PR: #806, Issue: #511) potentially refactor
                        inputRef={usernameRef}
                        id="username-textfield"
                        label={translate("Username")}
                        variant="outlined"
                        required
                        value={username}
                        error={usernameError}
                        disabled={disabled}
                        fullWidth
                        onChange={(v) => setUsername(v.target.value)}
                        onFocus={() => setUsernameError(false)}
                        autoCapitalize="none"
                        autoComplete="username"
                        onKeyPress={(ev) => {
                            if (ev.key === "Enter") {
                                if (!username.length) {
                                    setUsernameError(true);
                                } else if (username.length && password.length) {
                                    handleSignIn();
                                } else {
                                    setUsernameError(false);
                                    passwordRef.current.focus();
                                }
                            }
                        }}
                    />
                </Grid>
                <Grid item xs={12}>
                    <FixedTextField
                        // TODO (PR: #806, Issue: #511) potentially refactor
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
                        onKeyPress={(ev) => {
                            if (ev.key === "Enter") {
                                if (!username.length) {
                                    usernameRef.current.focus();
                                } else if (!password.length) {
                                    passwordRef.current.focus();
                                }
                                handleSignIn();
                                ev.preventDefault();
                            }
                        }}
                    />
                </Grid>
                {props.rememberMe ? (
                    <Grid item xs={12} sx={style.actionRow}>
                        <FormControlLabel
                            sx={style.rememberMe}
                            label={translate("Remember me", "Remember me")}
                            control={
                                <Checkbox
                                    id="remember-checkbox"
                                    disabled={disabled}
                                    checked={rememberMe}
                                    onChange={handleRememberMeChange}
                                    onKeyPress={(ev) => {
                                        if (ev.key === "Enter") {
                                            if (!username.length) {
                                                usernameRef.current.focus();
                                            } else if (!password.length) {
                                                passwordRef.current.focus();
                                            }
                                            handleSignIn();
                                        }
                                    }}
                                    value="rememberMe"
                                    color="primary"
                                />
                            }
                        />
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
                        {translate("Sign in")}
                    </Button>
                </Grid>
                {props.resetPassword ? (
                    <Grid item xs={12} sx={{ ...style.actionRow, ...style.flexEnd }}>
                        <Link
                            id="reset-password-button"
                            component="button"
                            onClick={handleResetPasswordClick}
                            sx={style.resetLink}
                            underline="hover"
                        >
                            {translate("Reset password?")}
                        </Link>
                    </Grid>
                ) : null}
            </Grid>
        </LoginLayout>
    );
};

export default FirstFactorForm;

const useStyles = (theme: Theme): { [key: string]: CSSProperties } => ({
    actionRow: {
        display: "flex",
        flexDirection: "row",
        marginTop: theme.spacing(-1),
        marginBottom: theme.spacing(-1),
    },
    resetLink: {
        cursor: "pointer",
        paddingTop: 13.5,
        paddingBottom: 13.5,
    },
    rememberMe: {
        flexGrow: 1,
    },
    flexEnd: {
        justifyContent: "flex-end",
    },
});
