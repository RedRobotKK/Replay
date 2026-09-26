package surface

// The provider contract facts, with the document each was read from.
//
// This file is the second of the two independent inputs. Nothing here was
// derived from a corpus, and nothing here may be. It is separated into its own
// file so that a reviewer disputing a classification can see at a glance which
// half of the argument they are disputing: the bytes on disk, or a claim about
// a provider's published pricing.
//
// A surface absent from this table is ContractUnknown, which is the safe
// default rather than an oversight. Adding a row is a claim that needs a
// source, and Validate refuses one without.

// AnthropicContract records that cache writes are billed above the uncached
// input rate, so a write is a distinct economic category.
var AnthropicContract = ContractFact{
	Write:  ContractPricedDistinctly,
	Source: "Anthropic prompt caching: cache writes billed at 1.25x (5m) and 2x (1h) of uncached input; rules document anthropic-2026-09-05",
}

// OpenAIModernContract records GPT-5.6 and later. Earlier generations carry no
// cache-write charge and are a DIFFERENT contract, which is why this variable
// names the generation rather than the vendor.
var OpenAIModernContract = ContractFact{
	Write:  ContractPricedDistinctly,
	Source: "developers.openai.com prompt-caching guide, read 2026-09-26: GPT-5.6 and later bill cache writes at 1.25x uncached input, reported as input_tokens_details.cache_write_tokens",
}

// OpenAILegacyContract records GPT-5.5 and earlier, which carry no cache-write
// charge.
var OpenAILegacyContract = ContractFact{
	Write:  ContractNotPricedDistinctly,
	Source: "developers.openai.com prompt-caching guide, read 2026-09-26: no additional cache-write charge before GPT-5.6",
}

// DeepSeekContract records that a miss bills as ordinary input, so there is no
// write premium to difference.
var DeepSeekContract = ContractFact{
	Write:  ContractNotPricedDistinctly,
	Source: "api-docs.deepseek.com context caching, read 2026-09-26: cache-hit and cache-miss input tokens priced separately, no cache-write charge",
}

// GeminiContract records that tokens used to create the cache bill at the
// standard input price, so there is no write premium to difference.
var GeminiContract = ContractFact{
	Write:  ContractNotPricedDistinctly,
	Source: "ai.google.dev context caching, read 2026-09-26: tokens used to create the cache bill at the standard input price",
}

// XAIContract is deliberately unknown.
//
// Grok's records carry a cacheCreationTokens counter that is zero on every one
// of them, beside cachedReadTokens that are not. That is the same shape Codex
// has, and on Codex it turned out to be a client dropping a counter the
// provider bills for. xAI's cache pricing was not found, so the shape alone
// does not say which world Grok is in, and this row says so rather than
// guessing from the zeros.
var XAIContract = ContractFact{Write: ContractUnknown}
