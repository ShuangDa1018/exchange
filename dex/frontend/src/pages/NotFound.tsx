import { Box, Button, Heading, Text } from "@chakra-ui/react";
import { useNavigate } from "react-router-dom";

export default function NotFound() {
  const nav = useNavigate();
  return (
    <Box minH="70vh" display="grid" placeItems="center">
      <Box textAlign="center">
        <Heading>404</Heading>
        <Text color="gray.400" my={3}>页面不存在</Text>
        <Button onClick={() => nav("/")}>回交易页</Button>
      </Box>
    </Box>
  );
}
