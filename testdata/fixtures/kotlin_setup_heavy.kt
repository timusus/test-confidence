package com.example

import org.junit.Test
import org.junit.Before
import org.junit.Rule
import io.mockk.MockK
import io.mockk.every
import io.mockk.coEvery
import io.mockk.verify
import io.mockk.mockk

class SetupHeavyTest {
    @MockK
    lateinit var repo: UserRepository

    @MockK
    lateinit var api: ApiClient

    @MockK
    lateinit var cache: CacheManager

    @MockK
    lateinit var logger: Logger

    @MockK
    lateinit var analytics: Analytics

    @Rule
    val testRule = InstantTaskExecutorRule()

    val testUser = User("John", 30)

    val testDispatcher = TestCoroutineDispatcher()

    @Before
    fun setUp() {
        MockKAnnotations.init(this)
        sut = UserViewModel(repo, api)
        cache.clear()
    }

    @Test
    fun `heavy setup with few assertions`() {
        every { repo.getUser() } returns testUser
        every { repo.getSettings() } returns settings
        every { api.fetchProfile() } returns profile
        every { api.fetchPrefs() } returns prefs
        every { cache.get("key1") } returns cachedValue1
        every { cache.get("key2") } returns cachedValue2
        every { logger.isEnabled() } returns true
        coEvery { analytics.track(any()) } returns Unit
        every { repo.getToken() } returns token
        every { api.validate(any()) } returns true
        val result = sut.loadUser()
        assertThat(result).isEqualTo(testUser)
    }

    @Test
    fun `another heavy test`() {
        every { repo.getUser() } returns testUser
        every { repo.getSettings() } returns settings
        every { api.fetchProfile() } returns profile
        coEvery { api.fetchPrefs() } returns prefs
        every { cache.get("key1") } returns cachedValue1
        every { cache.get("key2") } returns cachedValue2
        every { logger.isEnabled() } returns true
        coEvery { analytics.track(any()) } returns Unit
        val result = sut.process()
        assertThat(result).isEqualTo(expected)
    }

    @Test
    fun `balanced test`() {
        every { repo.getUser() } returns testUser
        val result = sut.getName()
        assertThat(result).isEqualTo("John")
        assertThat(result.length).isEqualTo(4)
    }
}
