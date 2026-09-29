"""Fixture tests for the Track C task classes. No credential, no network, no spend.

These tests are the only thing standing between "the instrument is sound" and
"the instrument agreed with itself". Three properties carry the weight:

  1. Ground truth is COMPUTED. Proven by changing the corpus and watching every
     dependent answer move. A hardcoded expected answer cannot pass this.
  2. Every checker accepts the true answer and rejects a plausible wrong one.
     The wrong answers used are the ones the model actually produced in
     reasoning-scope-2026-09-29.md, not invented ones.
  3. No question contains its own answer, and no question names its class.

Run: PYTHONDONTWRITEBYTECODE=1 python3 experiment/harness/test_taskclasses.py
"""
import inspect
import os
import re
import sys
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

import policy
import session
import taskclasses as tc
from fanout import Outcome
from policy import TaskClass


CORPUS = tc.load_corpus()
TASKS = tc.task_set(CORPUS)


def norm(s):
    """Lowercase, alphanumerics only. Deliberately aggressive: it collapses
    `hot-file` and `KindHotFile` onto the same string, which is the leak shape
    that Go's convention of naming a constant after its own value produces."""
    return re.sub(r"[^a-z0-9]", "", str(s).lower())


class TestTheCorpusIsRealSource(unittest.TestCase):

    def test_corpus_is_the_two_replay_files(self):
        for rel in tc.CORPUS_FILES:
            self.assertIn(tc.MARKER.format(rel), CORPUS)
        self.assertGreater(len(CORPUS), 40000, "corpus is too small to be the source")

    def test_corpus_agrees_with_the_earlier_measurement(self):
        """reasoning-scope-2026-09-29.md counted 39 `func ` lines and 14 `type `
        lines in this corpus. If those move, the corpus moved, and every figure
        quoted from that run against this one is comparing two documents."""
        self.assertEqual(tc.count_lines_starting(CORPUS, "func "), 39)
        self.assertEqual(tc.count_lines_starting(CORPUS, "type "), 14)

    def test_shared_block_survives_policy_assemble(self):
        """A prefix with leading whitespace caches nothing and policy refuses it."""
        self.assertTrue(policy.assemble(CORPUS, "q").startswith(CORPUS))


class TestGroundTruthIsComputedNotHardcoded(unittest.TestCase):
    """The load-bearing test. Each case perturbs the corpus and requires the
    answer to follow. Anything typed by hand stays still and fails here."""

    def test_counting_truth_follows_an_added_function(self):
        before = {t.label: t.ground_truth for t in TASKS}
        mutated = CORPUS + "\nfunc addedForTest() {}\n"
        after = {t.label: t.ground_truth for t in tc.task_set(mutated)}
        self.assertEqual(after["counting.func.lines"], before["counting.func.lines"] + 1)
        self.assertEqual(after["counting.type.lines"], before["counting.type.lines"])

    def test_lookup_and_derived_truths_follow_a_changed_constant(self):
        mutated = CORPUS.replace("const HashedLabelBytes = 12",
                                 "const HashedLabelBytes = 77")
        self.assertIn("const HashedLabelBytes = 77", mutated)
        after = {t.label: t.ground_truth for t in tc.task_set(mutated)}
        self.assertEqual(after["lookup.const.HashedLabelBytes"], 77)
        # the aggregation over three constants moves by the same delta
        before = {t.label: t.ground_truth for t in TASKS}
        self.assertEqual(after["aggregation.const.sum3"],
                         before["aggregation.const.sum3"] + 65)
        # and so does the transformation built on it
        self.assertEqual(after["transform.reverse.digits"], "77")

    def test_multistep_truth_follows_a_renamed_field(self):
        mutated = CORPUS.replace("\tFirstSeen     time.Time `json:\"first_seen\"`",
                                 "\tRenamedField  time.Time `json:\"first_seen\"`")
        self.assertNotEqual(mutated, CORPUS, "the fixture edit did not apply")
        after = {t.label: t.ground_truth for t in tc.task_set(mutated)}
        self.assertEqual(after["multistep.next.field"], "RenamedField")

    def test_transformation_truth_follows_a_renamed_function(self):
        mutated = CORPUS.replace("func SanitizeLabel(", "func SanitizeXy(")
        after = {t.label: t.ground_truth for t in tc.task_set(mutated)}
        self.assertEqual(after["transform.sort.funcname"], "".join(sorted("SanitizeXy")))

    def test_an_ambiguous_corpus_raises_instead_of_guessing(self):
        """Two functions matching a question that says "exactly one" makes the
        question unanswerable. A wrong answer to an unanswerable question is a
        harness failure being recorded as a model failure."""
        mutated = CORPUS + "\nfunc SanitizeSomethingElse(s string) string { return s }\n"
        with self.assertRaises(tc.CorpusAmbiguity):
            tc.task_set(mutated)


class TestEveryCheckerAcceptsTruthAndRejectsAPlausibleWrong(unittest.TestCase):

    def test_every_checker_accepts_its_own_ground_truth(self):
        for t in TASKS:
            with self.subTest(t.label):
                self.assertTrue(t.checker(str(t.ground_truth)),
                                f"{t.label} rejected its own truth {t.ground_truth!r}")

    def test_every_checker_accepts_the_truth_in_a_sentence(self):
        """Prompts say "and nothing else", but a model that prefixes one clause
        must not be scored wrong for prose. The rule is: last number, or last
        token of the last line."""
        for t in TASKS:
            with self.subTest(t.label):
                self.assertTrue(t.checker(f"The answer is {t.ground_truth}."),
                                f"{t.label} rejected its truth in a sentence")

    def test_every_checker_rejects_a_plausible_wrong_answer(self):
        """The wrong answers here are of the shape the model actually produced:
        a confident value of the right type, from the right document."""
        wrong = {
            "lookup.const.HashedLabelBytes": "400",
            "lookup.const.labelMaxLen": "12",
            "lookup.const.minInjectedTokens": "1000",
            "lookup.struct.owner": "Observation",
            "lookup.method.receiver": "Suggestion",
            # measured 2026-09-29: it answered 67 where the truth was 39, and 1
            # where the truth was 14
            "counting.func.lines": "67",
            "counting.type.lines": "1",
            "counting.rawmessage.occurrences": "6",
            "counting.struct.decls": "14",
            "aggregation.const.sum3": "10400",
            "aggregation.const.sumfloat": "1.20",
            "aggregation.const.mixed": "8788",
            "transform.reverse.funcname": "HashedPathLabel",
            "transform.sort.funcname": "SanitizeLabel",
            "transform.reverse.digits": "12",
            "multistep.next.field": "LastSeen",
            "multistep.receiver.lastfield": "targets",
            "multistep.ordinal.const": "Pending",
        }
        self.assertEqual(sorted(wrong), sorted(t.label for t in TASKS),
                         "a task was added or removed without a wrong answer for it")
        for t in TASKS:
            with self.subTest(t.label):
                self.assertNotEqual(str(t.ground_truth), wrong[t.label],
                                    "the wrong answer fixture equals the truth")
                self.assertFalse(t.checker(wrong[t.label]),
                                 f"{t.label} accepted {wrong[t.label]!r}")

    def test_literal_checkers_are_case_sensitive(self):
        """Go is case-sensitive, so `suggestion` is not `Suggestion`."""
        for t in TASKS:
            if not isinstance(t.ground_truth, str) or t.ground_truth.isdigit():
                continue
            with self.subTest(t.label):
                self.assertFalse(t.checker(t.ground_truth.swapcase()))

    def test_no_checker_raises_on_anything(self):
        """session.Run records passed=None when a checker raises, which is a paid
        call that measured nothing. Garbage is wrong, not undecided."""
        junk = ["", "   ", "\n\n", None, 0, [], "```\n```", chr(0x2014) * 2,
                "x" * 20000, "9" * 400, "NaN", "Infinity", "-", "1.2.3"]
        for t in TASKS:
            for j in junk:
                with self.subTest(label=t.label, junk=repr(j)[:20]):
                    self.assertIsInstance(t.checker(j), bool)


class TestNoQuestionLeaksItsOwnAnswer(unittest.TestCase):

    def test_no_question_contains_its_answer(self):
        """The corpus necessarily contains every LOOKUP answer; that is what
        makes it a lookup. What must not contain the answer is the QUESTION."""
        for t in TASKS:
            with self.subTest(t.label):
                self.assertNotIn(norm(t.ground_truth), norm(t.prompt_variable_part),
                                 f"{t.label} hands the model {t.ground_truth!r}")

    def test_no_question_names_its_class(self):
        """The class is the variable under study. A question that says "count"
        tells the model which machinery to use and measures something else."""
        words = ("lookup", "count", "aggregat", "transform", "multi-step",
                 "multistep", "step by step")
        for t in TASKS:
            low = t.prompt_variable_part.lower()
            for w in words:
                with self.subTest(label=t.label, word=w):
                    self.assertNotIn(w, low)

    def test_every_class_is_populated(self):
        have = {t.cls for t in TASKS}
        self.assertEqual(have, set(tc.CLASSES))
        for cls in tc.CLASSES:
            self.assertGreaterEqual(sum(1 for t in TASKS if t.cls == cls), 3, cls)

    def test_labels_are_unique(self):
        labels = [t.label for t in TASKS]
        self.assertEqual(len(labels), len(set(labels)))


class TestTheArmsReachTheSettingTheyClaim(unittest.TestCase):
    """session.Run.fan takes a task_class and no kwargs override, so an arm is
    realised by the task_class handed to it. If that stops being true, every
    reasoning figure in Track C is mislabelled."""

    def test_off_arm_disables_reasoning_for_every_class(self):
        for cls in tc.CLASSES:
            with self.subTest(cls):
                kw = policy.config_for(tc.policy_class_for(cls, tc.ARM_OFF))
                self.assertTrue(policy.disables_reasoning(kw))

    def test_on_arm_leaves_reasoning_enabled_for_every_class(self):
        for cls in tc.CLASSES:
            with self.subTest(cls):
                kw = policy.config_for(tc.policy_class_for(cls, tc.ARM_ON))
                self.assertFalse(policy.disables_reasoning(kw))

    def test_policy_arm_only_disables_reasoning_on_lookup(self):
        for cls in tc.CLASSES:
            kw = policy.config_for(tc.policy_class_for(cls, tc.ARM_POLICY))
            self.assertEqual(policy.disables_reasoning(kw), cls == tc.LOOKUP, cls)

    def test_unmeasured_transformation_defaults_to_the_expensive_setting(self):
        self.assertEqual(tc.POLICY_CLASS[tc.TRANSFORMATION], TaskClass.UNKNOWN)

    def test_forcing_an_arm_is_visible_to_policy_review(self):
        """The forced cell is an abuse of the enum. policy.review must say so;
        a silent forced run is one nobody can audit."""
        found = tc.review_plan(tc.COUNTING, CORPUS, tc.ARM_OFF)
        self.assertTrue(any("reasoning is disabled" in f for f in found), found)

    def test_unknown_arm_is_an_error_not_a_default(self):
        with self.assertRaises(ValueError):
            tc.policy_class_for(tc.LOOKUP, "cheap")


class TestItMatchesTheSessionInterface(unittest.TestCase):

    def test_fan_args_are_exactly_fan_parameters(self):
        params = set(inspect.signature(session.Run.fan).parameters) - {"self"}
        args = tc.fan_args([t for t in TASKS if t.cls == tc.LOOKUP], CORPUS, tc.ARM_OFF)
        self.assertTrue(set(args) <= params,
                        f"fan() does not take {sorted(set(args) - params)}")
        self.assertIn("task_class", args, "task_class has no default and is required")

    def test_fan_args_keeps_checks_aligned_with_variables(self):
        sub = [t for t in TASKS if t.cls == tc.COUNTING]
        args = tc.fan_args(sub, CORPUS, tc.ARM_ON)
        self.assertEqual(len(args["variables"]), len(args["checks"]))
        for chk, t in zip(args["checks"], sub):
            self.assertTrue(chk(str(t.ground_truth)))

    def test_one_fan_call_is_one_class(self):
        with self.assertRaises(ValueError):
            tc.fan_args(TASKS, CORPUS, tc.ARM_ON)

    def test_max_tokens_clears_the_measured_reasoning_budget(self):
        """The aggregative reasoning-ON cell averaged about 931 output tokens and
        an earlier run voided itself at 1024."""
        self.assertGreaterEqual(tc.MAX_TOKENS, 4096)


class TestUndecidedIsNeverScoredWrong(unittest.TestCase):

    def rows(self):
        return [
            {"id": 0, "outcome": Outcome.ANSWERED, "passed": True},
            {"id": 1, "outcome": Outcome.ANSWERED, "passed": False},
            {"id": 2, "outcome": Outcome.TRUNCATED, "passed": None},
            {"id": 3, "outcome": Outcome.REFUSED_BUDGET, "passed": None},
            {"id": 4, "outcome": Outcome.ERROR, "passed": None},
            {"id": 5, "outcome": Outcome.ANSWERED, "passed": None},
        ]

    def test_truncation_is_excluded_from_the_denominator(self):
        labels = {i: tc.COUNTING for i in range(6)}
        cell = tc.score_by_class(self.rows(), labels)[tc.COUNTING]
        self.assertEqual(cell["decided"], 2)
        self.assertEqual(cell["passed"], 1)
        self.assertEqual(cell["undecided"], 4)
        self.assertEqual(cell["accuracy"], 0.5)

    def test_a_checker_that_raised_is_undecided_not_wrong(self):
        """session.Run sets passed=None when a checker raises, on an ANSWERED
        row. Counting that as a failure would blame the model for the harness."""
        labels = {5: tc.COUNTING}
        cell = tc.score_by_class(self.rows(), labels)[tc.COUNTING]
        self.assertEqual(cell["decided"], 0)
        self.assertIsNone(cell["accuracy"])

    def test_a_cell_with_no_decided_answers_has_no_accuracy(self):
        labels = {2: tc.LOOKUP, 3: tc.LOOKUP}
        cell = tc.score_by_class(self.rows(), labels)[tc.LOOKUP]
        self.assertIsNone(cell["accuracy"], "an empty cell must not read as 0%")


if __name__ == "__main__":
    unittest.main(verbosity=1)
