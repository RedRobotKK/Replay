"""Fixture tests for prompt conditions. No credential, no network, no spend."""
import os, sys, unittest
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import promptcond, taskclasses


class TestParseIsFaithfulToTheBaseline(unittest.TestCase):
    def test_every_component_is_a_substring_of_its_baseline(self):
        """Components are PARSED, never transcribed. A component that is not a
        substring of P0 would mean the experiment compares against a prompt
        nobody ran."""
        for t in taskclasses.task_set():
            stmt, out, con = promptcond.parse(t.prompt_variable_part)
            for name, part in (("statement", stmt), ("output", out), ("constraint", con)):
                if part:
                    self.assertIn(part, t.prompt_variable_part,
                                  f"{t.label}: {name} is not in the baseline")

    def test_the_parse_is_non_trivial(self):
        """A parse that returned the whole prompt as the statement would make
        every condition a copy of P0 and the experiment vacuous."""
        for t in taskclasses.task_set():
            stmt, out, con = promptcond.parse(t.prompt_variable_part)
            self.assertTrue(out, f"{t.label}: no output format found")
            self.assertTrue(con, f"{t.label}: no closing constraint found")
            self.assertNotIn("Reply with", stmt, f"{t.label}: statement absorbed output")


class TestConditionsAreDistinctAndFaithful(unittest.TestCase):
    def test_p0_is_the_untouched_original(self):
        for t in taskclasses.task_set():
            self.assertEqual(promptcond.render("P0", t.prompt_variable_part),
                             t.prompt_variable_part)

    def test_every_condition_is_distinct_from_p0_except_p0(self):
        for t in taskclasses.task_set():
            p0 = t.prompt_variable_part
            for c in promptcond.CONDITIONS[1:]:
                self.assertNotEqual(promptcond.render(c, p0), p0,
                                    f"{t.label}/{c} is identical to the control")

    def test_no_condition_leaks_the_answer(self):
        """The failure that would invalidate everything."""
        for t in taskclasses.task_set():
            truth = str(t.ground_truth)
            if len(truth) < 3:
                continue          # a 2-char answer collides with ordinary text
            for c in promptcond.CONDITIONS:
                r = promptcond.render(c, t.prompt_variable_part)
                if truth in t.prompt_variable_part:
                    continue      # already in the baseline; not this code's doing
                self.assertNotIn(truth, r, f"{t.label}/{c} contains its own answer")

    def test_every_condition_keeps_the_task_statement(self):
        for t in taskclasses.task_set():
            stmt, _, _ = promptcond.parse(t.prompt_variable_part)
            for c in promptcond.CONDITIONS:
                self.assertIn(stmt, promptcond.render(c, t.prompt_variable_part),
                              f"{t.label}/{c} dropped the task statement")

    def test_p6_is_longer_than_p2_with_the_same_components(self):
        """The negative control only works if it is verbose AND structured."""
        for t in taskclasses.task_set():
            p0 = t.prompt_variable_part
            p2, p6 = promptcond.render("P2", p0), promptcond.render("P6", p0)
            self.assertGreater(len(p6), len(p2) * 1.5,
                               f"{t.label}: P6 is not materially more verbose than P2")
            stmt, _, _ = promptcond.parse(p0)
            self.assertIn(stmt, p6)

    def test_p1_is_shorter_than_p0(self):
        for t in taskclasses.task_set():
            p0 = t.prompt_variable_part
            self.assertLess(len(promptcond.render("P1", p0)), len(p0),
                            f"{t.label}: the concise condition is not shorter")


class TestEquivalenceReviewIsComputed(unittest.TestCase):
    def test_p1_declares_its_constraint_change(self):
        """P1 drops the closing constraint. That must be RECORDED, not excused."""
        p0 = taskclasses.task_set()[0].prompt_variable_part
        e = promptcond.equivalence("P1", p0)
        self.assertEqual(e["constraints_changed"], "YES")
        self.assertEqual(e["limited"], "YES")

    def test_structural_conditions_preserve_the_output_contract(self):
        p0 = taskclasses.task_set()[0].prompt_variable_part
        for c in ("P2", "P3", "P5", "P6"):
            e = promptcond.equivalence(c, p0)
            self.assertEqual(e["output_contract_changed"], "NO", c)
            self.assertEqual(e["semantic_content_changed"], "NO", c)


if __name__ == "__main__":
    unittest.main(verbosity=2)
