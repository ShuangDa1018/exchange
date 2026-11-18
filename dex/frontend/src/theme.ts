import { extendTheme, type ThemeConfig } from "@chakra-ui/react";

const config: ThemeConfig = { initialColorMode: "dark", useSystemColorMode: false };

const theme = extendTheme({
  config,
  fonts: { heading: `"IBM Plex Sans", sans-serif`, body: `"IBM Plex Sans", sans-serif` },
  colors: {
    bg: { 900: "#070a0d", 800: "#0d1218", 700: "#141b22" },
    up: { 500: "#17c784", 50: "#12352b" },
    down: { 500: "#f6465d", 50: "#3a151b" },
  },
  styles: {
    global: {
      body: { bg: "bg.900", color: "gray.100" },
    },
  },
  components: {
    Button: { defaultProps: { size: "sm" } },
  },
});

export default theme;
