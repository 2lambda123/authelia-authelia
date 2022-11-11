import React, { useEffect, useState } from "react";

import DeleteIcon from "@mui/icons-material/Delete";
import EditIcon from "@mui/icons-material/Edit";
import SystemSecurityUpdateGoodIcon from "@mui/icons-material/SystemSecurityUpdateGood";
import {
    AppBar,
    Box,
    Button,
    Drawer,
    Grid,
    IconButton,
    List,
    ListItem,
    ListItemButton,
    ListItemIcon,
    ListItemText,
    Paper,
    Stack,
    Switch,
    Table,
    TableBody,
    TableCell,
    TableHead,
    TableRow,
    Toolbar,
    Tooltip,
    Typography,
} from "@mui/material";
import { useNavigate } from "react-router-dom";

import { IndexRoute } from "@constants/Routes";
import { useNotifications } from "@hooks/NotificationsContext";
import { useAutheliaState } from "@hooks/State";
import { WebauthnDevice } from "@root/models/Webauthn";
import { getWebauthnDevices } from "@root/services/UserWebauthnDevices";
import { AuthenticationLevel } from "@services/State";

import AddSecurityKeyDialog from "./AddSecurityDialog";

interface Props {}

const drawerWidth = 240;

export default function SettingsView(props: Props) {
    const { createErrorNotification } = useNotifications();
    const navigate = useNavigate();
    const [webauthnDevices, setWebauthnDevices] = useState<WebauthnDevice[] | undefined>();
    const [addKeyOpen, setAddKeyOpen] = useState<boolean>(false);
    const [state, fetchState, , fetchStateError] = useAutheliaState();

    useEffect(() => {
        (async function () {
            const devices = await getWebauthnDevices();
            setWebauthnDevices(devices);
        })();
    }, []);

    useEffect(() => {
        if (state && state.authentication_level <= AuthenticationLevel.Unauthenticated) {
            navigate(IndexRoute);
        }
    }, [state, navigate]);

    // Fetch the state when portal is mounted.
    useEffect(() => {
        fetchState();
    }, [fetchState]);

    // Display an error when state fetching fails
    useEffect(() => {
        if (fetchStateError) {
            createErrorNotification("There was an issue fetching the current user state");
        }
    }, [fetchStateError, createErrorNotification]);

    const handleKeyClose = () => {
        setAddKeyOpen(false);
    };

    const handleAddKeyButtonClick = () => {
        setAddKeyOpen(true);
    };

    return (
        <Box sx={{ display: "flex" }}>
            <AppBar position="fixed" sx={{ zIndex: (theme) => theme.zIndex.drawer + 1 }}>
                <Toolbar variant="dense">
                    <Typography style={{ flexGrow: 1 }}>Settings</Typography>
                </Toolbar>
            </AppBar>
            <Drawer
                variant="permanent"
                sx={{
                    width: drawerWidth,
                    flexShrink: 0,
                    [`& .MuiDrawer-paper`]: { width: drawerWidth, boxSizing: "border-box" },
                }}
            >
                <Toolbar variant="dense" />
                <Box sx={{ overflow: "auto" }}>
                    <List>
                        <ListItem disablePadding>
                            <ListItemButton selected={true}>
                                <ListItemIcon>
                                    <SystemSecurityUpdateGoodIcon />
                                </ListItemIcon>
                                <ListItemText primary={"Security Keys"} />
                            </ListItemButton>
                        </ListItem>
                    </List>
                </Box>
            </Drawer>
            <Box component="main" sx={{ flexGrow: 1, p: 3 }}>
                <Grid container spacing={2}>
                    <Grid item xs={12}>
                        <Typography>Manage your security keys</Typography>
                    </Grid>
                    <Grid item xs={12}>
                        <Stack spacing={1} direction="row">
                            <Button color="primary" variant="contained" onClick={handleAddKeyButtonClick}>
                                Add
                            </Button>
                        </Stack>
                    </Grid>
                    <Grid item xs={12}>
                        <Paper>
                            <Table>
                                <TableHead>
                                    <TableRow>
                                        <TableCell>Name</TableCell>
                                        <TableCell>Enabled</TableCell>
                                        <TableCell>Activation</TableCell>
                                        <TableCell>Public Key</TableCell>
                                        <TableCell>Actions</TableCell>
                                    </TableRow>
                                </TableHead>
                                <TableBody>
                                    {webauthnDevices
                                        ? webauthnDevices.map((x, idx) => {
                                              return (
                                                  <TableRow key={x.description}>
                                                      <TableCell>{x.description}</TableCell>
                                                      <TableCell>
                                                          <Switch defaultChecked={false} size="small" />
                                                      </TableCell>
                                                      <TableCell>
                                                          <Typography>{false ? "<ADATE>" : "Not enabled"}</Typography>
                                                      </TableCell>
                                                      <TableCell>
                                                          <Tooltip title={x.public_key}>
                                                              <div
                                                                  style={{
                                                                      overflow: "hidden",
                                                                      textOverflow: "ellipsis",
                                                                      width: "300px",
                                                                  }}
                                                              >
                                                                  <Typography noWrap>{x.public_key}</Typography>
                                                              </div>
                                                          </Tooltip>
                                                      </TableCell>
                                                      <TableCell>
                                                          <Stack direction="row" spacing={1}>
                                                              <IconButton aria-label="edit">
                                                                  <EditIcon />
                                                              </IconButton>
                                                              <IconButton aria-label="delete">
                                                                  <DeleteIcon />
                                                              </IconButton>
                                                          </Stack>
                                                      </TableCell>
                                                  </TableRow>
                                              );
                                          })
                                        : null}
                                </TableBody>
                            </Table>
                        </Paper>
                    </Grid>
                </Grid>
            </Box>
            <AddSecurityKeyDialog open={addKeyOpen} onClose={handleKeyClose} />
        </Box>
    );
}
