import { useState } from "react";
import { Button, Card, Input, Label, TextField } from "@heroui/react";
import { api } from "../api";
import { LeafIcon } from "../components/icons";

interface LoginProps {
  onSuccess: () => void;
}

export default function Login({ onSuccess }: LoginProps) {
  const [key, setKey] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  const submit = async () => {
    if (!key.trim()) {
      setError("请输入访问密钥");
      return;
    }
    setLoading(true);
    setError("");
    try {
      await api.login(key.trim());
      onSuccess();
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="flex min-h-screen items-center justify-center bg-background px-6">
      <Card className="w-full max-w-sm">
        <Card.Header className="items-center text-center">
          <div className="mx-auto mb-3 flex size-12 items-center justify-center rounded-2xl bg-accent text-accent-foreground">
            <LeafIcon className="size-6" />
          </div>
          <Card.Title>AutoGreen</Card.Title>
          <Card.Description>请输入访问密钥以进入管理后台</Card.Description>
        </Card.Header>

        <Card.Content className="flex flex-col gap-4">
          <div>
            <TextField
              className="w-full"
              type="password"
              value={key}
              onChange={(value) => {
                setKey(value);
                setError("");
              }}
            >
              <Label>访问密钥</Label>
              <Input
                placeholder="48 位访问密钥"
                autoFocus
                onKeyDown={(event) => {
                  if (event.key === "Enter") void submit();
                }}
              />
            </TextField>
          </div>

          {error ? (
            <div className="rounded-xl bg-danger/10 px-3 py-2.5 text-sm leading-relaxed text-danger">
              {error}
            </div>
          ) : null}
        </Card.Content>

        <Card.Footer>
          <Button
            className="w-full"
            variant="primary"
            isPending={loading}
            onPress={submit}
          >
            登录
          </Button>
        </Card.Footer>
      </Card>
    </div>
  );
}
