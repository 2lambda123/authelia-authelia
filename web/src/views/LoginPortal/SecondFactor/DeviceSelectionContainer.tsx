import React, { CSSProperties, ReactNode, useState } from "react";

import { Typography, Grid, Button, Container, Theme, useTheme, Box } from "@mui/material";

import PushNotificationIcon from "@components/PushNotificationIcon";

export enum State {
    DEVICE = 1,
    METHOD = 2,
}

export interface SelectableDevice {
    id: string;
    name: string;
    methods: string[];
}

export interface SelectedDevice {
    id: string;
    method: string;
}

export interface Props {
    children?: ReactNode;
    devices: SelectableDevice[];

    onBack: () => void;
    onSelect: (device: SelectedDevice) => void;
}

const DefaultDeviceSelectionContainer = function (props: Props) {
    const [state, setState] = useState(State.DEVICE);
    const [device, setDevice] = useState([] as unknown as SelectableDevice);

    const handleDeviceSelected = (selecteddevice: SelectableDevice) => {
        if (selecteddevice.methods.length === 1) handleMethodSelected(selecteddevice.methods[0], selecteddevice.id);
        else {
            setDevice(selecteddevice);
            setState(State.METHOD);
        }
    };

    const handleMethodSelected = (method: string, deviceid?: string) => {
        if (deviceid) props.onSelect({ id: deviceid, method: method });
        else props.onSelect({ id: device.id, method: method });
    };

    let container: ReactNode;
    switch (state) {
        case State.DEVICE:
            container = (
                <Grid container justifyContent="center" spacing={1} id="device-selection">
                    {props.devices.map((value, index) => {
                        return (
                            <DeviceItem
                                id={index}
                                key={index}
                                device={value}
                                onSelect={() => handleDeviceSelected(value)}
                            />
                        );
                    })}
                </Grid>
            );
            break;
        case State.METHOD:
            container = (
                <Grid container justifyContent="center" spacing={1} id="method-selection">
                    {device.methods.map((value, index) => {
                        return (
                            <MethodItem
                                id={index}
                                key={index}
                                method={value}
                                onSelect={() => handleMethodSelected(value)}
                            />
                        );
                    })}
                </Grid>
            );
            break;
    }

    return (
        <Container>
            {container}
            <Button color="primary" onClick={props.onBack} id="device-selection-back">
                back
            </Button>
        </Container>
    );
};

export default DefaultDeviceSelectionContainer;

interface DeviceItemProps {
    id: number;
    device: SelectableDevice;

    onSelect: () => void;
}

function DeviceItem(props: DeviceItemProps) {
    const theme = useTheme();
    const style = useStyles(theme);

    const className = "device-option-" + props.id;
    const idName = "device-" + props.device.id;

    return (
        <Grid item xs={12} className={className} id={idName}>
            <Button
                sx={{ ...style.item, ...style.buttonRoot }}
                color="primary"
                variant="contained"
                onClick={props.onSelect}
            >
                <Box sx={style.icon}>
                    <PushNotificationIcon width={32} height={32} />
                </Box>
                <Box>
                    <Typography>{props.device.name}</Typography>
                </Box>
            </Button>
        </Grid>
    );
}

interface MethodItemProps {
    id: number;
    method: string;

    onSelect: () => void;
}

function MethodItem(props: MethodItemProps) {
    const theme = useTheme();
    const style = useStyles(theme);

    const className = "method-option-" + props.id;
    const idName = "method-" + props.method;

    return (
        <Grid item xs={12} className={className} id={idName}>
            <Button
                sx={{ ...style.item, ...style.buttonRoot }}
                color="primary"
                variant="contained"
                onClick={props.onSelect}
            >
                <Box sx={style.icon}>
                    <PushNotificationIcon width={32} height={32} />
                </Box>
                <Box>
                    <Typography>{props.method}</Typography>
                </Box>
            </Button>
        </Grid>
    );
}

const useStyles = (theme: Theme): { [key: string]: CSSProperties } => ({
    item: {
        paddingTop: theme.spacing(4),
        paddingBottom: theme.spacing(4),
        width: "100%",
    },
    icon: {
        display: "inline-block",
        fill: "white",
    },
    buttonRoot: {
        display: "block",
    },
});
