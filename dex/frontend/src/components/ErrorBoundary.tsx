import { Component, type ErrorInfo, type ReactNode } from "react";
import { Box, Button, Heading, Text } from "@chakra-ui/react";
import * as Sentry from "@sentry/react";

export class ErrorBoundary extends Component<{ children: ReactNode }, { err: Error | null }> {
  state = { err: null as Error | null };
  static getDerivedStateFromError(err: Error) { return { err }; }
  componentDidCatch(err: Error, info: ErrorInfo) {
    Sentry.captureException(err, { extra: { info } });
  }
  render() {
    if (!this.state.err) return this.props.children;
    return (
      <Box minH="100vh" display="grid" placeItems="center" px={6}>
        <Box maxW="md" textAlign="center">
          <Heading size="lg" mb={3}>页面崩溃了</Heading>
          <Text color="gray.400" mb={6} fontSize="sm">{this.state.err.message}</Text>
          <Button colorScheme="teal" onClick={() => location.assign("/")}>返回交易</Button>
        </Box>
      </Box>
    );
  }
}
