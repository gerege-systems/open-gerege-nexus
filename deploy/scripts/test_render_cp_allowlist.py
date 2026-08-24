import unittest

from render_cp_allowlist import parse_networks, render


class ControlPlaneAllowlistTests(unittest.TestCase):
    def test_renders_ipv4_ipv6_and_multiple_delimiters(self) -> None:
        output = render("203.0.113.10/32, 198.51.100.0/24\n2001:db8::/48")
        self.assertIn("allow 203.0.113.10/32;", output)
        self.assertIn("allow 198.51.100.0/24;", output)
        self.assertIn("allow 2001:db8::/48;", output)
        self.assertTrue(output.endswith("deny all;\n"))

    def test_accepts_single_addresses_and_removes_duplicates(self) -> None:
        self.assertEqual(parse_networks("203.0.113.10 203.0.113.10 ::1"), ["203.0.113.10", "::1"])

    def test_rejects_empty_invalid_and_injected_values(self) -> None:
        for raw in (
            "",
            "999.999.999.999/999",
            "203.0.113.5/24",
            "203.0.113.10; allow all",
            "cp.nexus.gerege.mn",
        ):
            with self.subTest(raw=raw), self.assertRaises(ValueError):
                parse_networks(raw)

    def test_rejects_default_routes_that_open_the_console_to_everyone(self) -> None:
        for raw in ("0.0.0.0/0", "::/0"):
            with self.subTest(raw=raw), self.assertRaises(ValueError):
                parse_networks(raw)


if __name__ == "__main__":
    unittest.main()
