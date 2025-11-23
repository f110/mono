import * as React from 'react'

import Drawer from '@mui/material/Drawer'

import Toolbar from '@mui/material/Toolbar'
import List from '@mui/material/List'

import Divider from '@mui/material/Divider'
import ListItem from '@mui/material/ListItem'
import ListItemButton from '@mui/material/ListItemButton'
import ListItemIcon from '@mui/material/ListItemIcon'
import ListItemText from '@mui/material/ListItemText'
import TaskIcon from '@mui/icons-material/Task'
import StorageIcon from '@mui/icons-material/Storage'
import InfoIcon from '@mui/icons-material/Info'
import { Link } from "@tanstack/react-router"

interface ListItemLinkProps {
    icon?: React.ReactElement<unknown>;
    primary: string;
    to: string;
}

function ListItemLink(props: ListItemLinkProps) {
    const {icon, primary, to} = props;

    return (
        <ListItemButton component={Link} to={to}>
            {icon ? <ListItemIcon>{icon}</ListItemIcon> : null}
            <ListItemText primary={primary}/>
        </ListItemButton>
    );
}

export const SideBar: React.FC = () => {
    const drawerWidth = 240;

    return (
        <Drawer
            sx={{
                width: drawerWidth,
                flexShrink: 0,
                '& .MuiDrawer-paper': {
                    width: drawerWidth,
                    boxSizing: 'border-box',
                },
            }}
            variant="permanent"
            anchor="left"
        >
            <Toolbar/>
            <Divider/>
            <List>
                <ListItem key='Task' disablePadding>
                    <ListItemLink primary='Task' to='/' icon={<TaskIcon/>}></ListItemLink>
                </ListItem>
                <ListItem key='Repositories' disablePadding>
                    <ListItemLink to='/repositories' primary='Repositories' icon={<StorageIcon/>}></ListItemLink>
                </ListItem>
                <ListItem key='Info' disablePadding>
                    <ListItemLink primary='Info' to='/info' icon={<InfoIcon/>}></ListItemLink>
                </ListItem>
            </List>
        </Drawer>
    )
}
